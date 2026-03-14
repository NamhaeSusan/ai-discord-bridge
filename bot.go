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

type Bot struct {
	session   *discordgo.Session
	cfg       *Config
	sessions  sync.Map // thread ID → sessionEntry
	semaphore chan struct{}
}

func NewBot(cfg *Config) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	bot := &Bot{
		session:   dg,
		cfg:       cfg,
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
	}

	dg.AddHandler(bot.onMessageCreate)
	return bot, nil
}

func (b *Bot) Open() error {
	go b.cleanupSessions()
	return b.session.Open()
}

func (b *Bot) Close() {
	b.session.Close()
}

func (b *Bot) cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	ttl := time.Duration(b.cfg.SessionTTLMinutes) * time.Minute
	for range ticker.C {
		b.sessions.Range(func(key, value any) bool {
			if entry, ok := value.(sessionEntry); ok {
				if time.Since(entry.createdAt) > ttl {
					b.sessions.Delete(key)
				}
			}
			return true
		})
	}
}

func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	if !b.isAllowed(m) {
		return
	}

	// Acquire semaphore (concurrency limit)
	select {
	case b.semaphore <- struct{}{}:
		defer func() { <-b.semaphore }()
	default:
		s.ChannelMessageSend(m.ChannelID, "Too many requests in progress. Please try again later.")
		return
	}

	ctx := context.Background()
	isThread := m.GuildID != "" && isThreadChannel(s, m.ChannelID)

	if isThread {
		// Message in existing thread → resume session
		b.handleThreadMessage(ctx, s, m)
	} else {
		// Message in regular channel → create thread + new session
		b.handleChannelMessage(ctx, s, m)
	}
}

func (b *Bot) handleChannelMessage(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) {
	// Create thread from the user's message
	threadName := truncate(m.Content, 100)
	thread, err := s.MessageThreadStartComplex(m.ChannelID, m.ID, &discordgo.ThreadStart{
		Name:                threadName,
		AutoArchiveDuration: 60,
	})
	if err != nil {
		log.Printf("failed to create thread: %v", err)
		return
	}

	done := make(chan struct{})
	go b.sendTyping(s, thread.ID, done)

	result, err := RunClaude(ctx, b.cfg, m.Content)
	close(done)

	if err != nil {
		log.Printf("claude error for user %s: %v", m.Author.ID, err)
		s.ChannelMessageSend(thread.ID, "Something went wrong. Check bot logs for details.")
		return
	}

	b.sendChunks(s, thread.ID, result)

	if result.SessionID != "" {
		b.sessions.Store(thread.ID, sessionEntry{
			sessionID: result.SessionID,
			createdAt: time.Now(),
		})
	}
}

func (b *Bot) handleThreadMessage(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) {
	done := make(chan struct{})
	go b.sendTyping(s, m.ChannelID, done)

	var result *ClaudeResult
	var err error

	if entry, ok := b.sessions.Load(m.ChannelID); ok {
		e := entry.(sessionEntry)
		result, err = ResumeClaude(ctx, b.cfg, e.sessionID, m.Content)
	} else {
		result, err = RunClaude(ctx, b.cfg, m.Content)
	}
	close(done)

	if err != nil {
		log.Printf("claude error for user %s: %v", m.Author.ID, err)
		s.ChannelMessageSend(m.ChannelID, "Something went wrong. Check bot logs for details.")
		return
	}

	b.sendChunks(s, m.ChannelID, result)

	if result.SessionID != "" {
		b.sessions.Store(m.ChannelID, sessionEntry{
			sessionID: result.SessionID,
			createdAt: time.Now(),
		})
	}
}

func (b *Bot) sendChunks(s *discordgo.Session, channelID string, result *ClaudeResult) {
	chunks := FormatResponse(result)
	for _, chunk := range chunks {
		if _, err := s.ChannelMessageSend(channelID, chunk); err != nil {
			log.Printf("failed to send message: %v", err)
			return
		}
	}
}

func (b *Bot) isAllowed(m *discordgo.MessageCreate) bool {
	channelOK := len(b.cfg.AllowedChannels) == 0
	for _, ch := range b.cfg.AllowedChannels {
		if ch == m.ChannelID {
			channelOK = true
			break
		}
	}

	userOK := len(b.cfg.AllowedUsers) == 0
	for _, u := range b.cfg.AllowedUsers {
		if u == m.Author.ID {
			userOK = true
			break
		}
	}

	return channelOK && userOK
}

func (b *Bot) sendTyping(s *discordgo.Session, channelID string, done <-chan struct{}) {
	s.ChannelTyping(channelID)
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			s.ChannelTyping(channelID)
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
