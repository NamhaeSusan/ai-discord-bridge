package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type sessionEntry struct {
	sessionID string
	createdAt time.Time
}

type Runner struct {
	session   *discordgo.Session
	cfg       BotConfig
	provider  Provider
	sessions  sync.Map
	semaphore chan struct{}
}

func NewBots(cfgs []BotConfig) ([]*Runner, error) {
	bots := make([]*Runner, 0, len(cfgs))
	for _, cfg := range cfgs {
		bot, err := NewBot(cfg)
		if err != nil {
			return nil, err
		}
		bots = append(bots, bot)
	}
	return bots, nil
}

func NewBot(cfg BotConfig) (*Runner, error) {
	dg, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return nil, err
	}

	provider, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	bot := &Runner{
		session:   dg,
		cfg:       cfg,
		provider:  provider,
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
	}

	dg.AddHandler(bot.onMessageCreate)
	return bot, nil
}

func (b *Runner) Open() error {
	go b.cleanupSessions()
	return b.session.Open()
}

func (b *Runner) Close() {
	b.session.Close()
}

func (b *Runner) cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	ttl := time.Duration(b.cfg.SessionTTLMinutes) * time.Minute
	for range ticker.C {
		b.sessions.Range(func(key, value any) bool {
			entry, ok := value.(sessionEntry)
			if ok && time.Since(entry.createdAt) > ttl {
				b.sessions.Delete(key)
			}
			return true
		})
	}
}

func (b *Runner) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot || !b.isAllowed(m) {
		return
	}

	if m.GuildID != "" && isThreadChannel(s, m.ChannelID) {
		if !b.ownsThread(s, m.ChannelID) {
			return
		}
	} else if !b.shouldHandleChannelMessage(m) {
		return
	}

	select {
	case b.semaphore <- struct{}{}:
		defer func() { <-b.semaphore }()
	default:
		s.ChannelMessageSend(m.ChannelID, "Too many requests in progress. Please try again later.")
		return
	}

	ctx := context.Background()
	if m.GuildID != "" && isThreadChannel(s, m.ChannelID) {
		b.handleThreadMessage(ctx, s, m)
		return
	}

	b.handleChannelMessage(ctx, s, m)
}

func (b *Runner) handleChannelMessage(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) {
	thread, err := s.MessageThreadStartComplex(m.ChannelID, m.ID, &discordgo.ThreadStart{
		Name:                truncate(m.Content, 100),
		AutoArchiveDuration: 60,
	})
	if err != nil {
		log.Printf("[%s] create thread: %v", b.cfg.Name, err)
		return
	}

	done := make(chan struct{})
	go b.sendTyping(s, thread.ID, done)

	result, err := b.provider.Run(ctx, m.Content)
	close(done)
	if err != nil {
		log.Printf("[%s] provider error for user %s: %v", b.cfg.Name, m.Author.ID, err)
		s.ChannelMessageSend(thread.ID, "Something went wrong. Check bot logs for details.")
		return
	}

	b.sendChunks(s, thread.ID, result)
	b.storeSession(thread.ID, result.SessionID)
}

func (b *Runner) handleThreadMessage(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) {
	done := make(chan struct{})
	go b.sendTyping(s, m.ChannelID, done)

	result, err := b.runThreadMessage(ctx, m.ChannelID, m.Content)
	close(done)
	if err != nil {
		log.Printf("[%s] provider error for user %s: %v", b.cfg.Name, m.Author.ID, err)
		s.ChannelMessageSend(m.ChannelID, "Something went wrong. Check bot logs for details.")
		return
	}

	b.sendChunks(s, m.ChannelID, result)
	b.storeSession(m.ChannelID, result.SessionID)
}

func (b *Runner) runThreadMessage(ctx context.Context, channelID, prompt string) (*ProviderResult, error) {
	entry, ok := b.sessions.Load(channelID)
	if !ok {
		return b.provider.Run(ctx, prompt)
	}

	return b.provider.Resume(ctx, entry.(sessionEntry).sessionID, prompt)
}

func (b *Runner) storeSession(channelID, sessionID string) {
	if sessionID == "" {
		return
	}

	b.sessions.Store(channelID, sessionEntry{
		sessionID: sessionID,
		createdAt: time.Now(),
	})
}

func (b *Runner) sendChunks(s *discordgo.Session, channelID string, result *ProviderResult) {
	for _, chunk := range FormatResponse(result) {
		if _, err := s.ChannelMessageSend(channelID, chunk); err != nil {
			log.Printf("[%s] send message: %v", b.cfg.Name, err)
			return
		}
	}
}

func (b *Runner) isAllowed(m *discordgo.MessageCreate) bool {
	return matchesChannelFilter(b.session, b.cfg.AllowedChannels, m.ChannelID) && matchesFilter(b.cfg.AllowedUsers, m.Author.ID)
}

func (b *Runner) sendTyping(s *discordgo.Session, channelID string, done <-chan struct{}) {
	_ = s.ChannelTyping(channelID)
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			_ = s.ChannelTyping(channelID)
		}
	}
}

func isThreadChannel(s *discordgo.Session, channelID string) bool {
	ch, err := s.Channel(channelID)
	if err != nil {
		return false
	}
	return ch.IsThread()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return fmt.Sprintf("%s...", s[:maxLen-3])
}

func matchesFilter(allowed []string, actual string) bool {
	if len(allowed) == 0 {
		return true
	}

	for _, item := range allowed {
		if item == actual {
			return true
		}
	}

	return false
}

func matchesChannelFilter(s *discordgo.Session, allowed []string, channelID string) bool {
	if len(allowed) == 0 {
		return true
	}

	if matchesFilter(allowed, channelID) {
		return true
	}

	ch, err := s.Channel(channelID)
	if err != nil || ch == nil || ch.ParentID == "" {
		return false
	}

	return matchesFilter(allowed, ch.ParentID)
}

func (b *Runner) shouldHandleChannelMessage(m *discordgo.MessageCreate) bool {
	if m.GuildID == "" {
		return true
	}

	botUser := b.session.State.User
	if botUser == nil {
		return false
	}

	for _, mention := range m.Mentions {
		if mention != nil && mention.ID == botUser.ID {
			return true
		}
	}

	return false
}

func (b *Runner) ownsThread(s *discordgo.Session, channelID string) bool {
	ch, err := s.Channel(channelID)
	if err != nil || ch == nil {
		return false
	}

	botUser := b.session.State.User
	if botUser == nil {
		return false
	}

	return ch.OwnerID == botUser.ID
}
