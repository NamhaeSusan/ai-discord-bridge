package main

import (
	"context"
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
	sessions  sync.Map // bot reply msg ID → sessionEntry
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
		s.ChannelMessageSendReply(m.ChannelID, "Too many requests in progress. Please try again later.", m.Reference())
		return
	}

	ctx := context.Background()

	// Start typing indicator in background
	done := make(chan struct{})
	go b.sendTyping(s, m.ChannelID, done)

	var result *ClaudeResult
	var err error

	if m.MessageReference != nil {
		if entry, ok := b.sessions.Load(m.MessageReference.MessageID); ok {
			e := entry.(sessionEntry)
			result, err = ResumeClaude(ctx, b.cfg, e.sessionID, m.Content)
		} else {
			result, err = RunClaude(ctx, b.cfg, m.Content)
		}
	} else {
		result, err = RunClaude(ctx, b.cfg, m.Content)
	}

	close(done)

	if err != nil {
		log.Printf("claude error for user %s: %v", m.Author.ID, err)
		s.ChannelMessageSendReply(m.ChannelID, "Something went wrong. Check bot logs for details.", m.Reference())
		return
	}

	chunks := FormatResponse(result)
	var lastMsgID string
	for _, chunk := range chunks {
		msg, sendErr := s.ChannelMessageSendReply(m.ChannelID, chunk, m.Reference())
		if sendErr != nil {
			log.Printf("failed to send message: %v", sendErr)
			return
		}
		lastMsgID = msg.ID
	}

	if lastMsgID != "" && result.SessionID != "" {
		b.sessions.Store(lastMsgID, sessionEntry{
			sessionID: result.SessionID,
			createdAt: time.Now(),
		})
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
