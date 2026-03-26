package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

func (b *Runner) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}

	action, ownerID, err := parseCwdComponentID(i.MessageComponentData().CustomID)
	if err != nil {
		return
	}

	userID := interactionUserID(i)
	if userID == "" {
		return
	}
	if userID != ownerID {
		b.respondInteractionMessage(s, i, discordgo.InteractionResponseChannelMessageWithSource, "This `/cwd` picker belongs to another user.", true)
		return
	}

	currentDir := b.currentWorkingDir(i.ChannelID)
	switch action {
	case cwdComponentRefreshButton:
		b.respondCwdPickerUpdate(s, i, currentDir)
	case cwdComponentAliasSelect:
		values := i.MessageComponentData().Values
		if len(values) != 1 {
			b.respondInteractionMessage(s, i, discordgo.InteractionResponseChannelMessageWithSource, "Select one alias.", true)
			return
		}
		dir, err := b.resolveWorkingDir(i.ChannelID, values[0])
		if err != nil {
			b.respondInteractionMessage(s, i, discordgo.InteractionResponseChannelMessageWithSource, err.Error(), true)
			return
		}
		b.updateSessionWorkingDir(i.ChannelID, dir)
		b.recordRecentDir(dir)
		b.respondCwdPickerUpdate(s, i, dir)
	case cwdComponentRecentSelect:
		values := i.MessageComponentData().Values
		if len(values) != 1 {
			b.respondInteractionMessage(s, i, discordgo.InteractionResponseChannelMessageWithSource, "Select one recent path.", true)
			return
		}
		index, err := strconv.Atoi(values[0])
		if err != nil {
			b.respondInteractionMessage(s, i, discordgo.InteractionResponseChannelMessageWithSource, "Invalid recent path selection.", true)
			return
		}
		dir, err := b.recentDir(index)
		if err != nil {
			b.respondInteractionMessage(s, i, discordgo.InteractionResponseChannelMessageWithSource, err.Error(), true)
			return
		}
		b.updateSessionWorkingDir(i.ChannelID, dir)
		b.recordRecentDir(dir)
		b.respondCwdPickerUpdate(s, i, dir)
	default:
		b.respondInteractionMessage(s, i, discordgo.InteractionResponseChannelMessageWithSource, "Unknown `/cwd` action.", true)
	}
}

func (b *Runner) handleChannelCwdCommand(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate, cmd cwdCommand) {
	switch cmd.Kind {
	case cwdCommandShow:
		thread, err := b.startCwdThread(s, m.ChannelID, m.ID, b.currentWorkingDir(""), "")
		if err != nil {
			log.Printf("[%s] create cwd thread: %v", b.cfg.Name, err)
			return
		}
		b.threads.Claim(thread.ID, b.cfg.Name)
		b.sendCwdPicker(s, thread.ID, m.Author.ID)
	case cwdCommandSet:
		dir, err := b.resolveWorkingDir("", cmd.Target)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, err.Error())
			return
		}

		thread, err := b.startCwdThread(s, m.ChannelID, m.ID, dir, cmd.Target)
		if err != nil {
			log.Printf("[%s] create cwd thread: %v", b.cfg.Name, err)
			return
		}
		b.storeSession(thread.ID, "", dir)
		b.recordRecentDir(dir)
		if cmd.Prompt == "" {
			b.sendCwdPicker(s, thread.ID, m.Author.ID)
			return
		}
		b.runNewThreadPrompt(ctx, s, thread.ID, m.Author.ID, cmd.Prompt, dir)
	case cwdCommandRecentList:
		s.ChannelMessageSend(m.ChannelID, formatRecentList(b.listRecentDirs()))
	case cwdCommandRecentUse:
		dir, err := b.recentDir(cmd.Index)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, err.Error())
			return
		}

		thread, err := b.startCwdThread(s, m.ChannelID, m.ID, dir, fmt.Sprintf("recent %d", cmd.Index))
		if err != nil {
			log.Printf("[%s] create cwd thread: %v", b.cfg.Name, err)
			return
		}
		b.storeSession(thread.ID, "", dir)
		b.recordRecentDir(dir)
		b.sendCwdPicker(s, thread.ID, m.Author.ID)
	case cwdCommandAliasList:
		s.ChannelMessageSend(m.ChannelID, formatAliasList(b.listAliases()))
	case cwdCommandAliasAdd:
		if !b.canManageAliases(s, m.GuildID, m.ChannelID, m.Author.ID) {
			s.ChannelMessageSend(m.ChannelID, "Alias management requires Discord administrator permissions in a server channel.")
			return
		}
		dir, err := b.resolveWorkingDir("", cmd.Path)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, err.Error())
			return
		}
		if err := b.store.PutAlias(b.cfg.Name, cmd.Alias, dir); err != nil {
			log.Printf("[%s] persist alias: %v", b.cfg.Name, err)
			s.ChannelMessageSend(m.ChannelID, "Failed to save alias.")
			return
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Alias `%s` -> `%s` saved.", cmd.Alias, dir))
	case cwdCommandAliasRemove:
		if !b.canManageAliases(s, m.GuildID, m.ChannelID, m.Author.ID) {
			s.ChannelMessageSend(m.ChannelID, "Alias management requires Discord administrator permissions in a server channel.")
			return
		}
		if err := b.store.DeleteAlias(b.cfg.Name, cmd.Alias); err != nil {
			log.Printf("[%s] delete alias: %v", b.cfg.Name, err)
			s.ChannelMessageSend(m.ChannelID, "Failed to delete alias.")
			return
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Alias `%s` removed.", cmd.Alias))
	}
}

func (b *Runner) handleThreadCwdCommand(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate, cmd cwdCommand) {
	switch cmd.Kind {
	case cwdCommandShow:
		b.sendCwdPicker(s, m.ChannelID, m.Author.ID)
	case cwdCommandSet:
		dir, err := b.resolveWorkingDir(m.ChannelID, cmd.Target)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, err.Error())
			return
		}
		b.updateSessionWorkingDir(m.ChannelID, dir)
		b.recordRecentDir(dir)
		if cmd.Prompt == "" {
			b.sendCwdPicker(s, m.ChannelID, m.Author.ID)
			return
		}
		b.runExistingThreadPrompt(ctx, s, m.ChannelID, m.Author.ID, cmd.Prompt, dir)
	case cwdCommandRecentList:
		s.ChannelMessageSend(m.ChannelID, formatRecentList(b.listRecentDirs()))
		b.sendCwdPicker(s, m.ChannelID, m.Author.ID)
	case cwdCommandRecentUse:
		dir, err := b.recentDir(cmd.Index)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, err.Error())
			return
		}
		b.updateSessionWorkingDir(m.ChannelID, dir)
		b.recordRecentDir(dir)
		b.sendCwdPicker(s, m.ChannelID, m.Author.ID)
	case cwdCommandAliasList:
		s.ChannelMessageSend(m.ChannelID, formatAliasList(b.listAliases()))
		b.sendCwdPicker(s, m.ChannelID, m.Author.ID)
	case cwdCommandAliasAdd:
		if !b.canManageAliases(s, m.GuildID, m.ChannelID, m.Author.ID) {
			s.ChannelMessageSend(m.ChannelID, "Alias management requires Discord administrator permissions in a server channel.")
			return
		}
		dir, err := b.resolveWorkingDir(m.ChannelID, cmd.Path)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, err.Error())
			return
		}
		if err := b.store.PutAlias(b.cfg.Name, cmd.Alias, dir); err != nil {
			log.Printf("[%s] persist alias: %v", b.cfg.Name, err)
			s.ChannelMessageSend(m.ChannelID, "Failed to save alias.")
			return
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Alias `%s` -> `%s` saved.", cmd.Alias, dir))
		b.sendCwdPicker(s, m.ChannelID, m.Author.ID)
	case cwdCommandAliasRemove:
		if !b.canManageAliases(s, m.GuildID, m.ChannelID, m.Author.ID) {
			s.ChannelMessageSend(m.ChannelID, "Alias management requires Discord administrator permissions in a server channel.")
			return
		}
		if err := b.store.DeleteAlias(b.cfg.Name, cmd.Alias); err != nil {
			log.Printf("[%s] delete alias: %v", b.cfg.Name, err)
			s.ChannelMessageSend(m.ChannelID, "Failed to delete alias.")
			return
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Alias `%s` removed.", cmd.Alias))
		b.sendCwdPicker(s, m.ChannelID, m.Author.ID)
	}
}

func (b *Runner) runNewThreadPrompt(ctx context.Context, s *discordgo.Session, channelID, userID, prompt, workingDir string) {
	done := make(chan struct{})
	go b.sendTyping(s, channelID, done)

	result, err := b.provider.Run(ctx, prompt, workingDir)
	close(done)
	if err != nil {
		log.Printf("[%s] provider error for user %s: %v", b.cfg.Name, userID, err)
		s.ChannelMessageSend(channelID, "Something went wrong. Check bot logs for details.")
		return
	}

	result.WorkingDir = effectiveWorkingDir(b.cfg, workingDir)
	b.sendChunks(s, channelID, result)
	b.storeSession(channelID, result.SessionID, workingDir)
}

func (b *Runner) runExistingThreadPrompt(ctx context.Context, s *discordgo.Session, channelID, userID, prompt, workingDir string) {
	done := make(chan struct{})
	go b.sendTyping(s, channelID, done)

	result, sessionChanged, err := b.runThreadMessage(ctx, channelID, prompt, workingDir)
	close(done)
	if err != nil {
		log.Printf("[%s] provider error for user %s: %v", b.cfg.Name, userID, err)
		s.ChannelMessageSend(channelID, "Something went wrong. Check bot logs for details.")
		return
	}

	if sessionChanged {
		s.ChannelMessageSend(channelID, "> ⚠️ Session changed — previous context was compacted or lost. Responses below may lack earlier context.")
	}

	result.WorkingDir = effectiveWorkingDir(b.cfg, workingDir)
	b.sendChunks(s, channelID, result)
	b.storeSession(channelID, result.SessionID, workingDir)
}

func (b *Runner) sendCwdPicker(s *discordgo.Session, channelID, userID string) {
	if _, err := s.ChannelMessageSendComplex(channelID, buildCwdPickerMessage(b.currentWorkingDir(channelID), b.listAliases(), b.listRecentDirs(), userID)); err != nil {
		log.Printf("[%s] send cwd picker: %v", b.cfg.Name, err)
	}
}

func (b *Runner) respondCwdPickerUpdate(s *discordgo.Session, i *discordgo.InteractionCreate, dir string) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    cwdPickerMessage(dir),
			Components: buildCwdComponents(dir, b.listAliases(), b.listRecentDirs(), interactionUserID(i)),
		},
	}); err != nil {
		log.Printf("[%s] update cwd picker: %v", b.cfg.Name, err)
	}
}

func (b *Runner) respondInteractionMessage(s *discordgo.Session, i *discordgo.InteractionCreate, responseType discordgo.InteractionResponseType, content string, ephemeral bool) {
	data := &discordgo.InteractionResponseData{
		Content: content,
	}
	if ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: responseType,
		Data: data,
	}); err != nil {
		log.Printf("[%s] interaction response: %v", b.cfg.Name, err)
	}
}

func (b *Runner) currentWorkingDir(channelID string) string {
	if channelID != "" {
		if entry, ok := b.sessions.Load(channelID); ok {
			if dir := entry.(sessionEntry).workingDir; dir != "" {
				return dir
			}
		}
	}
	return b.cfg.WorkingDir
}

func (b *Runner) resolveWorkingDir(channelID, target string) (string, error) {
	dir, err := resolveCwdTarget(target, b.resolveBaseDir(channelID), b.listAliases())
	if err != nil {
		return "", err
	}
	if err := validateWorkingDir(b.cfg.WorkingDir, dir); err != nil {
		return "", err
	}
	return dir, nil
}

func (b *Runner) resolveBaseDir(channelID string) string {
	if dir := b.currentWorkingDir(channelID); dir != "" {
		return dir
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func (b *Runner) recordRecentDir(dir string) {
	if dir == "" || b.store == nil {
		return
	}
	if err := b.store.PutRecentDir(b.cfg.Name, dir); err != nil {
		log.Printf("[%s] persist recent dir: %v", b.cfg.Name, err)
	}
}

func (b *Runner) listAliases() map[string]string {
	if b.store == nil {
		return map[string]string{}
	}
	return b.store.ListAliases(b.cfg.Name)
}

func (b *Runner) listRecentDirs() []string {
	if b.store == nil {
		return nil
	}
	return b.store.ListRecentDirs(b.cfg.Name)
}

func (b *Runner) recentDir(index int) (string, error) {
	recents := b.listRecentDirs()
	if index <= 0 || index > len(recents) {
		return "", fmt.Errorf("recent path `%d` not found", index)
	}
	dir, err := ensureDirectory(recents[index-1])
	if err != nil {
		return "", err
	}
	if err := validateWorkingDir(b.cfg.WorkingDir, dir); err != nil {
		return "", err
	}
	return dir, nil
}

func (b *Runner) canManageAliases(s *discordgo.Session, guildID, channelID, userID string) bool {
	if guildID == "" || b.store == nil {
		return false
	}
	perms, err := s.UserChannelPermissions(userID, channelID)
	if err != nil {
		log.Printf("[%s] resolve channel permissions: %v", b.cfg.Name, err)
		return false
	}
	return perms&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator
}

func (b *Runner) startCwdThread(s *discordgo.Session, channelID, messageID, displayDir, label string) (*discordgo.Channel, error) {
	name := "/cwd"
	switch {
	case displayDir != "":
		name = fmt.Sprintf("/cwd %s", displayDir)
	case label != "":
		name = "/cwd " + label
	}
	return s.MessageThreadStartComplex(channelID, messageID, &discordgo.ThreadStart{
		Name:                truncate(name, 100),
		AutoArchiveDuration: 60,
	})
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}
