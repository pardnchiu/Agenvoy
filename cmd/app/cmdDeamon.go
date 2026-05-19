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
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	geminiStt "github.com/pardnchiu/agenvoy/internal/agents/provider/gemini/stt"
	geminiYoutube "github.com/pardnchiu/agenvoy/internal/agents/provider/gemini/youtube"
	codexImage2 "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex/image2"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/routes"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/discord"
	discordTool "github.com/pardnchiu/agenvoy/internal/runtime/discord/tool"
	"github.com/pardnchiu/agenvoy/internal/runtime/telegram"
	telegramTool "github.com/pardnchiu/agenvoy/internal/runtime/telegram/tool"
	"github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/tools/agent/subagent"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
)

var (
	discordMu          sync.Mutex
	discordBot         *discord.Bot
	lastDiscordEnabled bool
	lastDiscordToken   string

	telegramMu          sync.Mutex
	telegramBot         *telegram.Bot
	lastTelegramEnabled bool
	lastTelegramToken   string
)

func reloadDiscord() {
	newToken := keychain.Get(discord.Key)
	newEnabled := false
	if cfg, err := session.Load(); err == nil && cfg != nil {
		newEnabled = cfg.DiscordEnabled
	}

	discordMu.Lock()
	defer discordMu.Unlock()

	if newEnabled == lastDiscordEnabled && newToken == lastDiscordToken {
		return
	}

	if discordBot != nil {
		_ = discord.Close(discordBot)
		discordBot = nil
	}
	lastDiscordEnabled = newEnabled
	lastDiscordToken = newToken

	if !newEnabled {
		slog.Info("discord disabled, skipped")
		return
	}
	if newToken == "" {
		slog.Info("discord enabled but token missing, run `/discord enable` in TUI")
		return
	}

	bot, err := discord.New()
	if err != nil {
		slog.Error("discord.New",
			slog.String("error", err.Error()))
		return
	}
	discordBot = bot
}

func reloadTelegram() {
	newToken := keychain.Get(telegram.Key)
	newEnabled := false
	if cfg, err := session.Load(); err == nil && cfg != nil {
		newEnabled = cfg.TelegramEnabled
	}

	telegramMu.Lock()
	defer telegramMu.Unlock()

	if newEnabled == lastTelegramEnabled && newToken == lastTelegramToken {
		return
	}

	if telegramBot != nil {
		_ = telegram.Close(telegramBot)
		telegramBot = nil
	}
	lastTelegramEnabled = newEnabled
	lastTelegramToken = newToken

	if !newEnabled {
		slog.Info("telegram disabled, skipped")
		return
	}
	if newToken == "" {
		slog.Info("telegram enabled but token missing, run `/telegram enable` in TUI")
		return
	}

	bot, err := telegram.New()
	if err != nil {
		slog.Error("telegram.New",
			slog.String("error", err.Error()))
		return
	}
	telegramBot = bot
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

	codexImage2.Register()
	geminiYoutube.Register()
	geminiStt.Register()
	telegramTool.Register()
	discordTool.Register()

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
	scanner := runtime.NewSkillScanner()
	selectorBot := plannerSelector(registry)

	agents.Set(selectorBot, registry, scanner)
	agents.SetRefresher(refreshHost)

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
		reloadDiscord()
		reloadTelegram()

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

		go setSummaryCron()
	} else {
		slog.Warn("no agents configured, server, discord, and telegram disabled")
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
	telegramMu.Lock()
	if telegramBot != nil {
		_ = telegram.Close(telegramBot)
		telegramBot = nil
	}
	telegramMu.Unlock()
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
				if agents.Reload() {
					slog.Info("host reloaded from config change")
				}
				reloadDiscord()
				reloadTelegram()
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

	output, err := exec.ExecWithSubagent(exec.WithDcPushPrefix(ctx, skillName), body, sessionID, "", "", nil)
	if err != nil {
		return "", err
	}

	return output, nil
}
