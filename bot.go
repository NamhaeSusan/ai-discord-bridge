package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type sessionEntry struct {
	sessionID    string
	workingDir   string
	lastAccessAt time.Time
}

type Runner struct {
	session   *discordgo.Session
	cfg       BotConfig
	provider  Provider
	threads   *threadRegistry
	sessions  sync.Map
	store     *sessionStore
	semaphore chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

func NewBots(cfgs []BotConfig, store *sessionStore) ([]*Runner, error) {
	registry := newThreadRegistry(store)
	bots := make([]*Runner, 0, len(cfgs))
	for _, cfg := range cfgs {
		bot, err := NewBot(cfg, registry, store)
		if err != nil {
			return nil, err
		}
		bots = append(bots, bot)
	}
	return bots, nil
}

func NewBot(cfg BotConfig, registry *threadRegistry, store *sessionStore) (*Runner, error) {
	dg, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session for bot %q", cfg.Name)
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
		threads:   registry,
		store:     store,
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
		done:      make(chan struct{}),
	}

	dg.AddHandler(bot.onMessageCreate)
	return bot, nil
}

func (b *Runner) Open() error {
	if b.store != nil {
		for ch, entry := range b.store.AllSessions(b.cfg.Name) {
			b.sessions.Store(ch, entry)
		}
	}
	go b.cleanupSessions()
	return b.session.Open()
}

func (b *Runner) Close() {
	b.closeOnce.Do(func() {
		close(b.done)
		b.session.Close()
	})
}

func (b *Runner) cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	ttl := time.Duration(b.cfg.SessionTTLMinutes) * time.Minute
	for {
		select {
		case <-b.done:
			return
		case <-ticker.C:
			b.sessions.Range(func(key, value any) bool {
				entry, ok := value.(sessionEntry)
				if ok && time.Since(entry.lastAccessAt) > ttl {
					b.notifySessionExpired(key.(string))
					b.sessions.Delete(key)
				}
				return true
			})
			if b.store != nil {
				if err := b.store.PurgeExpiredSessions(b.cfg.Name, ttl); err != nil {
					log.Printf("[%s] purge expired sessions: %v", b.cfg.Name, err)
				}
			}
		}
	}
}

func (b *Runner) notifySessionExpired(channelID string) {
	if _, err := b.session.ChannelMessageSend(channelID, "Session expired due to inactivity. Start a new conversation to continue."); err != nil {
		log.Printf("[%s] notify expired session in thread %s: %v", b.cfg.Name, channelID, err)
	}

	archived := true
	locked := true
	if _, err := b.session.ChannelEdit(channelID, &discordgo.ChannelEdit{
		Archived: &archived,
		Locked:   &locked,
	}); err != nil {
		log.Printf("[%s] close expired thread %s: %v", b.cfg.Name, channelID, err)
	}

	ch, err := b.session.Channel(channelID)
	if err != nil || ch.ParentID == "" {
		return
	}
	if _, err := b.session.ChannelMessageSend(ch.ParentID, fmt.Sprintf("Session in thread <#%s> has expired and the thread was closed.", channelID)); err != nil {
		log.Printf("[%s] notify parent channel for thread %s: %v", b.cfg.Name, channelID, err)
	}
}

func (b *Runner) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot || !b.isAllowed(m) {
		return
	}

	isThread := m.GuildID != "" && isThreadChannel(s, m.ChannelID)
	if isThread {
		if !b.shouldHandleThreadMessage(m) {
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
	if isThread {
		b.handleThreadMessage(ctx, s, m)
		return
	}

	b.handleChannelMessage(ctx, s, m)
}

func (b *Runner) handleChannelMessage(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) {
	content := stripBotMention(m.Content, s.State.User.ID)
	workingDir, prompt, err := parseWorkdirDirective(content)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, err.Error())
		return
	}

	if workingDir != "" {
		if err := validateWorkingDir(b.cfg.WorkingDir, workingDir); err != nil {
			s.ChannelMessageSend(m.ChannelID, err.Error())
			return
		}
	}

	if prompt == "" {
		displayDir := workingDir
		if displayDir == "" {
			displayDir = b.cfg.WorkingDir
		}
		threadName := "/cwd"
		if displayDir != "" {
			threadName = fmt.Sprintf("/cwd %s", displayDir)
		}
		thread, err := s.MessageThreadStartComplex(m.ChannelID, m.ID, &discordgo.ThreadStart{
			Name:                truncate(threadName, 100),
			AutoArchiveDuration: 60,
		})
		if err != nil {
			log.Printf("[%s] create thread: %v", b.cfg.Name, err)
			return
		}
		if workingDir != "" {
			b.storeSession(thread.ID, "", workingDir)
			s.ChannelMessageSend(thread.ID, fmt.Sprintf("Working directory set to `%s`", workingDir))
		} else {
			b.threads.Claim(thread.ID, b.cfg.Name)
			s.ChannelMessageSend(thread.ID, cwdDisplayMessage(displayDir))
		}
		return
	}

	thread, err := s.MessageThreadStartComplex(m.ChannelID, m.ID, &discordgo.ThreadStart{
		Name:                truncate(prompt, 100),
		AutoArchiveDuration: 60,
	})
	if err != nil {
		log.Printf("[%s] create thread: %v", b.cfg.Name, err)
		return
	}

	done := make(chan struct{})
	go b.sendTyping(s, thread.ID, done)

	result, err := b.provider.Run(ctx, prompt, workingDir)
	close(done)
	if err != nil {
		log.Printf("[%s] provider error for user %s: %v", b.cfg.Name, m.Author.ID, err)
		s.ChannelMessageSend(thread.ID, "Something went wrong. Check bot logs for details.")
		return
	}

	effectiveDir := workingDir
	if effectiveDir == "" {
		effectiveDir = b.cfg.WorkingDir
	}
	result.WorkingDir = effectiveDir
	b.sendChunks(s, thread.ID, result)
	b.storeSession(thread.ID, result.SessionID, workingDir)
}

func (b *Runner) handleThreadMessage(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) {
	content := stripBotMention(m.Content, s.State.User.ID)
	workingDir, prompt, err := parseWorkdirDirective(content)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, err.Error())
		return
	}

	if workingDir != "" {
		if err := validateWorkingDir(b.cfg.WorkingDir, workingDir); err != nil {
			s.ChannelMessageSend(m.ChannelID, err.Error())
			return
		}
	}

	if prompt == "" {
		if workingDir == "" {
			displayDir := b.cfg.WorkingDir
			if entry, ok := b.sessions.Load(m.ChannelID); ok {
				displayDir = entry.(sessionEntry).workingDir
			}
			s.ChannelMessageSend(m.ChannelID, cwdDisplayMessage(displayDir))
		} else {
			b.updateSessionWorkingDir(m.ChannelID, workingDir)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Working directory set to `%s`", workingDir))
		}
		return
	}

	done := make(chan struct{})
	go b.sendTyping(s, m.ChannelID, done)

	result, sessionChanged, err := b.runThreadMessage(ctx, m.ChannelID, prompt, workingDir)
	close(done)
	if err != nil {
		log.Printf("[%s] provider error for user %s: %v", b.cfg.Name, m.Author.ID, err)
		s.ChannelMessageSend(m.ChannelID, "Something went wrong. Check bot logs for details.")
		return
	}

	if sessionChanged {
		s.ChannelMessageSend(m.ChannelID, "> ⚠️ Session changed — previous context was compacted or lost. Responses below may lack earlier context.")
	}

	effectiveDir := workingDir
	if effectiveDir == "" {
		if entry, ok := b.sessions.Load(m.ChannelID); ok {
			effectiveDir = entry.(sessionEntry).workingDir
		}
	}
	if effectiveDir == "" {
		effectiveDir = b.cfg.WorkingDir
	}
	result.WorkingDir = effectiveDir
	b.sendChunks(s, m.ChannelID, result)
	b.storeSession(m.ChannelID, result.SessionID, effectiveDir)
}

func (b *Runner) runThreadMessage(ctx context.Context, channelID, prompt, workingDir string) (result *ProviderResult, sessionChanged bool, err error) {
	entry, ok := b.sessions.Load(channelID)
	if !ok {
		log.Printf("[%s] thread %s: no existing session, starting new", b.cfg.Name, channelID)
		r, err := b.provider.Run(ctx, prompt, workingDir)
		return r, false, err
	}

	session := entry.(sessionEntry)
	if workingDir == "" {
		workingDir = session.workingDir
	}
	if session.sessionID == "" {
		log.Printf("[%s] thread %s: stored session has empty ID, starting new", b.cfg.Name, channelID)
		r, err := b.provider.Run(ctx, prompt, workingDir)
		return r, false, err
	}

	r, err := b.provider.Resume(ctx, session.sessionID, prompt, workingDir)
	if err != nil {
		log.Printf("[%s] thread %s: resume %s failed: %v — starting new session", b.cfg.Name, channelID, session.sessionID, err)
		r, err = b.provider.Run(ctx, prompt, workingDir)
		return r, true, err
	}

	if r.SessionID != session.sessionID {
		log.Printf("[%s] thread %s: session changed %s -> %s (context compaction)", b.cfg.Name, channelID, session.sessionID, r.SessionID)
		return r, true, nil
	}

	return r, false, nil
}

func (b *Runner) updateSessionWorkingDir(channelID, workingDir string) {
	var updated sessionEntry
	if entry, ok := b.sessions.Load(channelID); ok {
		updated = entry.(sessionEntry)
		updated.workingDir = workingDir
		updated.lastAccessAt = time.Now()
	} else {
		updated = sessionEntry{
			workingDir:   workingDir,
			lastAccessAt: time.Now(),
		}
	}
	b.sessions.Store(channelID, updated)
	if b.store != nil {
		if err := b.store.PutSession(b.cfg.Name, channelID, updated); err != nil {
			log.Printf("[%s] persist session: %v", b.cfg.Name, err)
		}
	}
}

func (b *Runner) storeSession(channelID, sessionID, workingDir string) {
	entry := sessionEntry{
		sessionID:    sessionID,
		workingDir:   workingDir,
		lastAccessAt: time.Now(),
	}
	b.sessions.Store(channelID, entry)
	b.threads.Claim(channelID, b.cfg.Name)
	if b.store != nil {
		if err := b.store.PutSession(b.cfg.Name, channelID, entry); err != nil {
			log.Printf("[%s] persist session: %v", b.cfg.Name, err)
		}
	}
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

func cwdDisplayMessage(dir string) string {
	if dir == "" {
		return "Working directory not set. Use `/cwd <path>` to set it."
	}
	return fmt.Sprintf("Working directory: `%s`", dir)
}

func stripBotMention(content string, botID string) string {
	for _, prefix := range []string{"<@!" + botID + ">", "<@" + botID + ">"} {
		if strings.HasPrefix(content, prefix) {
			return strings.TrimSpace(content[len(prefix):])
		}
	}
	return content
}

func isThreadChannel(s *discordgo.Session, channelID string) bool {
	ch, err := s.Channel(channelID)
	if err != nil {
		return false
	}
	return ch.IsThread()
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

func matchesFilter(allowed []string, actual string) bool {
	if len(allowed) == 0 {
		return true
	}

	return slices.Contains(allowed, actual)
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

func (b *Runner) shouldHandleThreadMessage(m *discordgo.MessageCreate) bool {
	owner, claimed := b.threads.Owner(m.ChannelID)
	if claimed {
		return owner == b.cfg.Name
	}

	return b.shouldHandleChannelMessage(m)
}

type threadRegistry struct {
	m     sync.Map
	store *sessionStore
}

func newThreadRegistry(store *sessionStore) *threadRegistry {
	r := &threadRegistry{store: store}
	if store != nil {
		for ch, owner := range store.AllThreads() {
			r.m.Store(ch, owner)
		}
	}
	return r
}

func (r *threadRegistry) Claim(channelID, botName string) {
	if channelID == "" || botName == "" {
		return
	}

	r.m.Store(channelID, botName)
	if r.store != nil {
		if err := r.store.PutThread(channelID, botName); err != nil {
			log.Printf("persist thread ownership: %v", err)
		}
	}
}

func (r *threadRegistry) Owner(channelID string) (string, bool) {
	if channelID == "" {
		return "", false
	}

	owner, ok := r.m.Load(channelID)
	if !ok {
		return "", false
	}

	name, ok := owner.(string)
	if !ok || name == "" {
		return "", false
	}

	return name, true
}

func parseWorkdirDirective(content string) (string, string, error) {
	if content == "" {
		return "", "", fmt.Errorf("message cannot be empty")
	}
	lines := strings.Split(content, "\n")

	first := lines[0]
	if len(first) == 0 || first[:1] != "/" {
		return "", content, nil
	}

	const prefix = "/cwd "
	if first == "/cwd" {
		return "", "", nil
	}
	if len(first) < len(prefix) || first[:len(prefix)] != prefix {
		return "", content, nil
	}

	dir := first[len(prefix):]
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("invalid `/cwd` path: %v", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return "", "", fmt.Errorf("working directory not found: `%s`", absDir)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("working directory is not a directory: `%s`", absDir)
	}

	prompt := joinLines(lines[1:])
	return absDir, prompt, nil
}

func validateWorkingDir(baseDir, targetDir string) error {
	if baseDir == "" {
		return nil
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("invalid base working directory: %v", err)
	}
	if targetDir == absBase {
		return nil
	}
	if !strings.HasPrefix(targetDir, absBase+string(filepath.Separator)) {
		return fmt.Errorf("path `%s` is outside allowed base directory `%s`", targetDir, absBase)
	}
	return nil
}

func joinLines(lines []string) string {
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
