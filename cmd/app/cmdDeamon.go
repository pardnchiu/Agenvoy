package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/torii"
	"github.com/pardnchiu/agenvoy/internal/routes"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/discord"
	discordTypes "github.com/pardnchiu/agenvoy/internal/runtime/discord/types"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
	"github.com/pardnchiu/agenvoy/internal/tools/agent/subagent"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
)

var (
	discordMu          sync.Mutex
	discordBot         *discordTypes.DiscordBot
	lastDiscordEnabled bool
	lastDiscordToken   string
	lastDiscordGuild   string
)

func reloadDiscord() {
	newToken := keychain.Get(discord.Key)
	newEnabled := false
	newGuild := ""
	if cfg, err := session.Load(); err == nil && cfg != nil {
		newEnabled = cfg.DiscordEnabled
		newGuild = cfg.DiscordGuildID
	}

	discordMu.Lock()
	defer discordMu.Unlock()

	if newEnabled == lastDiscordEnabled && newToken == lastDiscordToken && newGuild == lastDiscordGuild {
		return
	}

	if discordBot != nil {
		_ = discord.Close(discordBot)
		discordBot = nil
	}
	lastDiscordEnabled = newEnabled
	lastDiscordToken = newToken
	lastDiscordGuild = newGuild

	if !newEnabled {
		slog.Info("discord disabled, skipped")
		return
	}
	if newToken == "" {
		slog.Info("discord enabled but token missing, run `agen discord enable`")
		return
	}

	bot, err := discord.New()
	if err != nil {
		slog.Error("discord.New",
			slog.String("error", err.Error()))
		return
	}
	discordBot = bot
	slog.Info("discord reloaded")
}

func cmdDaemon() {
	session.SetHash(session.Hash())

	if err := filesystem.Init(); err != nil {
		slog.Error("filesystem.Init",
			slog.String("error", err.Error()))
		return
	}
	if err := torii.Init(filesystem.StoreDir); err != nil {
		slog.Error("store.Init",
			slog.String("error", err.Error()))
		return
	}
	defer torii.Close()

	if _, err := runtime.Init(); err != nil {
		if errors.Is(err, runtime.ErrAlreadyRunning) {
			slog.Error("daemon already running, aborting")
			return
		}
		slog.Warn("runtime.Init",
			slog.String("error", err.Error()))
	}
	session.Clean()
	session.CleanAllTask()

	if err := go_pkg_sandbox.CheckDependence(); err != nil {
		slog.Error("sandbox.CheckDependence",
			slog.String("error", err.Error()))
	}

	if cfg, err := session.Load(); err == nil {
		provider.SetReasoningLevel(cfg.ReasoningLevel)
	}
	subagent.Register()

	mcpManager := initMCP(context.Background())
	defer mcpManager.Close()

	registry := buildAgentRegistry()
	scanner := skill.NewScanner()
	selectorBot := plannerSelector(registry)

	host.Set(selectorBot, registry, scanner)
	host.SetRefresher(refreshHost)

	runtime.SetRunner(runSkill)
	if err := runtime.NewScheduler(); err != nil {
		slog.Error("runtime.SchedulerInit",
			slog.String("error", err.Error()))
	}
	defer runtime.StopScheduler()

	stopSchedulerWatcher := runtime.SchedulerWatcher(context.Background())
	defer stopSchedulerWatcher()

	stopWatcher := watchConfig(context.Background())
	defer stopWatcher()

	var server *http.Server

	if selectorBot != nil {
		slog.Info("agent registry built",
			slog.Int("entries", len(registry.Entries)),
			slog.String("fallback", selectorBot.Name()))

		reloadDiscord()

		route := routes.New()
		port := go_pkg_utils.GetWithDefault("PORT", "17989")
		server = &http.Server{
			Addr:    ":" + port,
			Handler: route,
		}

		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("server.ListenAndServe",
					slog.String("error", err.Error()))
			}
		}()
		slog.Info("server started",
			slog.String("port", port))

		go setSummaryCron()
	} else {
		slog.Warn("no agents configured, server and discord disabled")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("daemon shutting down")

	discordMu.Lock()
	if discordBot != nil {
		_ = discord.Close(discordBot)
		discordBot = nil
	}
	discordMu.Unlock()
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = server.Shutdown(ctx)
		cancel()
	}
	if err := runtime.Clear(); err != nil {
		slog.Warn("runtime.Clear",
			slog.String("error", err.Error()))
	}
}

func watchConfig(ctx context.Context) func() {
	configDir := filepath.Dir(filesystem.ConfigPath)
	configBase := filepath.Base(filesystem.ConfigPath)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("watchConfig.NewWatcher",
			slog.String("error", err.Error()))
		return func() {}
	}
	if err := w.Add(configDir); err != nil {
		slog.Warn("watchConfig.Add",
			slog.String("dir", configDir),
			slog.String("error", err.Error()))
		_ = w.Close()
		return func() {}
	}

	stopCh := make(chan struct{})
	go func() {
		defer w.Close()
		var lastReload time.Time
		for {
			select {
			case <-stopCh:
				return
			case <-ctx.Done():
				return
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				if filepath.Base(ev.Name) != configBase {
					continue
				}
				if !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Create) && !ev.Has(fsnotify.Rename) {
					continue
				}
				if time.Since(lastReload) < 200*time.Millisecond {
					continue
				}
				lastReload = time.Now()
				if cfg, err := session.Load(); err == nil {
					provider.SetReasoningLevel(cfg.ReasoningLevel)
				}
				if host.Reload() {
					slog.Info("host reloaded from config change")
				}
				reloadDiscord()
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				slog.Warn("watchConfig",
					slog.String("error", err.Error()))
			}
		}
	}()
	return func() { close(stopCh) }
}

func runSkill(ctx context.Context, sessionID, skillName string) (string, error) {
	body, err := filesystem.ScheduleSkillBody(skillName)
	if err != nil {
		return "", fmt.Errorf("scheduler skill %q unreadable: %w", skillName, err)
	}
	sessionDir := filepath.Join(filesystem.SessionsDir, sessionID)
	if err := go_pkg_filesystem.CheckDir(sessionDir, true); err != nil {
		return "", err
	}
	session.SaveBot(sessionID, sessionID, false)

	output, err := exec.ExecWithSubagent(ctx, body, sessionID, "", "", nil)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(sessionID, "dc-") {
		body := stripSubagentHeader(output)
		if body != "" {
			message := fmt.Sprintf("%s\n-# shchedule | %s", body, skillName)
			discordMu.Lock()
			bot := discordBot
			discordMu.Unlock()
			if bot != nil && bot.Session != nil {
				channelID, cerr := session.GetChannelID(sessionID)
				if cerr != nil {
					slog.Warn("scheduler.runSkill: GetChannelID",
						slog.String("session_id", sessionID),
						slog.String("error", cerr.Error()))
				} else if channelID != "" {
					if _, derr := bot.Session.ChannelMessageSend(channelID, message); derr != nil {
						slog.Warn("scheduler.runSkill: ChannelMessageSend",
							slog.String("channel_id", channelID),
							slog.String("error", derr.Error()))
					}
				}
			}
		}
	}

	return output, nil
}

func stripSubagentHeader(out string) string {
	trimmed := strings.TrimSpace(out)
	if !strings.HasPrefix(trimmed, "[subagent") {
		return trimmed
	}
	_, rest, found := strings.Cut(trimmed, "\n")
	if !found {
		return ""
	}
	return strings.TrimSpace(rest)
}
