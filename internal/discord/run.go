package discord

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	discordTypes "github.com/pardnchiu/agenvoy/internal/discord/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func run(ctx context.Context, dcBot *discordTypes.DiscordBot, dcSession *discordgo.Session, dcMessageCreate *discordgo.MessageCreate, receiveMessage *discordTypes.ReceiveMessage) error {
	workDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("os.UserHomeDir: %w", err)
	}

	dcBot.SkillScanner.Scan()

	fileNames := make([]string, len(receiveMessage.FileInputs))
	for i, f := range receiveMessage.FileInputs {
		fileNames[i] = f.Name
	}
	skill := exec.SelectSkill(ctx, dcBot.PlannerAgent, dcBot.SkillScanner, receiveMessage.Content, fileNames)
	if skill != nil {
		slog.Info("skill", slog.String("skill", skill.Name))
	}
	agent := exec.SelectAgent(ctx, dcBot.PlannerAgent, dcBot.AgentRegistry, receiveMessage.Content, skill != nil)

	execData := exec.ExecData{
		Agent:   agent,
		WorkDir: workDir,
		Skill:   skill,
		Content: receiveMessage.Content,
	}

	session, err := getSession(ctx, dcSession, receiveMessage.GuildID, receiveMessage.ChannelID, receiveMessage.AuthorID, dcMessageCreate.ID, receiveMessage.Content, receiveMessage.ImageInputs, receiveMessage.FileInputs, execData)
	if err != nil {
		return fmt.Errorf("loadDiscordSession: %w", err)
	}
	utils.EventLog("[Discord]", agentTypes.Event{}, session.ID, strings.TrimSpace(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(receiveMessage.Content, "")))

	interactionMax := 128
	if skill == nil {
		interactionMax = 16
	}
	events := make(chan agentTypes.Event, interactionMax)

	go func() {
		err := exec.Execute(ctx, execData, session, events, true)
		if err != nil {
			slog.Warn("exec.Execute",
				slog.String("error", err.Error()))
		}
		close(events)
	}()

	var replyText string
	var execErrors []string
	var doneEvent agentTypes.Event
	for e := range events {
		utils.EventLog("[Discord]", e, session.ID, "")
		switch e.Type {
		case agentTypes.EventText:
			replyText = e.Text

		case agentTypes.EventExecError:
			slog.Warn("EventExecError",
				slog.String("tool", e.ToolName),
				slog.String("hash", e.Text))
			execErrors = append(execErrors, fmt.Sprintf("`%s` → `%s`", e.ToolName, e.Text))

		case agentTypes.EventDone:
			doneEvent = e
		// * use full name for remindering
		case agentTypes.EventSkillSelect,
			agentTypes.EventSkillResult,
			agentTypes.EventAgentSelect,
			agentTypes.EventAgentResult,
			agentTypes.EventToolCall,
			agentTypes.EventToolCallStart,
			agentTypes.EventToolCallEnd,
			agentTypes.EventToolConfirm,
			agentTypes.EventToolCallText,
			agentTypes.EventToolResult,
			agentTypes.EventToolSkipped:
			break
		}
	}

	replyText = strings.TrimSpace(regexp.MustCompile(`^ts:\d+\n`).ReplaceAllString(replyText, ""))
	if replyText == "" {
		return fmt.Errorf("no reply")
	}

	fileMarker := regexp.MustCompile(`\[SEND_FILE:([^\]]+)\]`)
	var filePaths []string
	for _, match := range fileMarker.FindAllStringSubmatch(replyText, -1) {
		filePaths = append(filePaths, strings.TrimSpace(match[1]))
	}
	replyText = strings.TrimSpace(fileMarker.ReplaceAllString(replyText, ""))

	model := doneEvent.Model
	if model == "" {
		model = agent.Name()
	}
	footer := model
	if doneEvent.Usage != nil {
		footer = fmt.Sprintf("%s | in:%d out:%d", footer, doneEvent.Usage.Input, doneEvent.Usage.Output)
	}
	replyText = fmt.Sprintf("%s\n-# %s", replyText, footer)
	if len(execErrors) > 0 {
		replyText = fmt.Sprintf("%s\n-# ⚠️ %s", replyText, strings.Join(execErrors, ", "))
	}

	dr := &discordTypes.DiscordReply{
		Session:   dcSession,
		ChannelID: dcMessageCreate.ChannelID,
		Reference: dcMessageCreate.Reference(),
	}
	if err := Reply(ctx, dr, discordTypes.ReplyMessage{
		Content:   replyText,
		FilePaths: filePaths,
	}); err != nil {
		slog.Warn("ReplyDiscord",
			slog.String("error", err.Error()))
	}

	return nil
}
