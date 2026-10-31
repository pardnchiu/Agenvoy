package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"

	"github.com/pardnchiu/agenvoy/internal/agents"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/app"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/record"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	chatbottool "github.com/pardnchiu/agenvoy/internal/runtime/chatbot/tool"
	"github.com/pardnchiu/agenvoy/internal/runtime/ipc"
	"github.com/pardnchiu/agenvoy/internal/runtime/mcp"
	"github.com/pardnchiu/agenvoy/internal/runtime/pubsub"
	"github.com/pardnchiu/agenvoy/internal/runtime/routes"
	"github.com/pardnchiu/agenvoy/internal/runtime/routes/handler"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	"github.com/pardnchiu/agenvoy/internal/runtime/webapp"
	"github.com/pardnchiu/agenvoy/internal/session"
	sessionlog "github.com/pardnchiu/agenvoy/internal/session/log"
	sessiontui "github.com/pardnchiu/agenvoy/internal/session/tui"
	usagelog "github.com/pardnchiu/agenvoy/internal/session/usage"
	audiotool "github.com/pardnchiu/agenvoy/internal/tools/external/audio"
	imagetool "github.com/pardnchiu/agenvoy/internal/tools/external/image"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	"github.com/pardnchiu/agenvoy/internal/tools/subagent"
)

func Daemon() {
	bootAt := time.Now()
	bootPhase := func(name string) {
		slog.Debug("boot phase",
			slog.String("phase", name),
			slog.Duration("elapsed", time.Since(bootAt)))
	}

	app.InstallDaemonLog()
	sessiontui.New()

	if err := filesystem.Init(); err != nil {
		slog.Error("filesystem.Init",
			slog.String("error", err.Error()))
		return
	}
	if err := filesystem.LoadRuntime(); err != nil {
		slog.Warn("filesystem.LoadRuntime",
			slog.String("error", err.Error()))
	}
	if err := record.TrimLog(); err != nil {
		slog.Warn("record TrimLog",
			slog.String("error", err.Error()))
	}
	if err := torii.Init(filesystem.StoreDir); err != nil {
		slog.Error("store.Init",
			slog.String("error", err.Error()))
		return
	}
	defer torii.Close()

	if n := interactive.ClearOnline(); n > 0 {
		slog.Info("cleared stale in-flight markers",
			slog.Int("count", n))
	}

	if err := filesystem.OpenDB(); err != nil {
		slog.Error("filesystem.OpenDB",
			slog.String("error", err.Error()))
		return
	}
	defer filesystem.CloseDB()

	if err := historystore.New(); err != nil {
		slog.Warn("historystore.New",
			slog.String("error", err.Error()))
	}
	defer historystore.Close()

	if err := usagelog.New(); err != nil {
		slog.Warn("usagelog.New",
			slog.String("error", err.Error()))
	}
	defer usagelog.Close()

	bootPhase("storage")

	imagetool.Register()
	audiotool.Register()
	chatbottool.Register()

	if _, err := runtime.Init(); err != nil {
		if errors.Is(err, runtime.ErrAlreadyRunning) {
			slog.Error("daemon already running, aborting")
			return
		}
		slog.Warn("runtime.Init",
			slog.String("error", err.Error()))
	}

	if err := webapp.SyncAsset(context.Background()); err != nil {
		slog.Warn("webapp.SyncAsset",
			slog.String("error", err.Error()))
	}

	if path, err := webapp.Install(context.Background()); err != nil {
		slog.Warn("webapp.Install",
			slog.String("error", err.Error()))
	} else {
		slog.Debug("webapp.Install",
			slog.String("path", path))
	}

	if err := go_pkg_sandbox.CheckDependence(); err != nil {
		slog.Error("sandbox.CheckDependence",
			slog.String("error", err.Error()))
	}

	subagent.Register()
	subagent.SetDeliver(app.DeliverBackground)

	defer func() {
		if m := mcp.Manager(); m != nil {
			m.Close()
		}
	}()

	agents.Set(nil, nil, agenttypes.ModelSet{}, runtime.NewSkillScanner())
	agents.SetRefresher(app.RefreshHost)

	go func() {
		mcp.SetManager(app.NewMCP(context.Background(), ""))
		bootPhase("mcp ready")
	}()

	runtime.SetRunner(app.RunSkill)
	if err := runtime.NewScheduler(); err != nil {
		slog.Error("runtime.SchedulerInit",
			slog.String("error", err.Error()))
	}
	defer runtime.StopScheduler()

	app.RecoverBackground()

	if err := runtime.AddSystemCron("*/15 * * * *", app.GenerateSummary); err != nil {
		slog.Warn("cron summaryGenerate",
			slog.String("error", err.Error()))
	}

	if err := runtime.AddSystemCron("*/30 * * * *", session.Clean); err != nil {
		slog.Warn("cron sessionClean",
			slog.String("error", err.Error()))
	}

	stopSchedulerWatcher := runtime.SchedulerWatcher(context.Background())
	defer stopSchedulerWatcher()

	stopWatcher := app.WatchConfig(context.Background(), func() {
		if agents.Reload() {
			slog.Debug("host reloaded: config change")
		}
		app.ReloadDiscord(0)
		app.ReloadTelegram(0)
	})
	defer stopWatcher()

	stopSessionWatcher := app.WatchSession(context.Background())
	defer stopSessionWatcher()

	app.ReloadDiscord(0)
	app.ReloadTelegram(0)

	handler.StartWebConfirm(context.Background())

	stopIPC, err := ipc.Listen(context.Background())
	if err != nil {
		slog.Error("ipc.Listen",
			slog.String("socket", filesystem.DaemonSocketPath),
			slog.String("error", err.Error()))
		return
	}
	defer stopIPC()

	runtime.RegisterCancelNotifier(func(sessionID, taskHash, reason string) {
		event := agenttypes.Event{Type: agenttypes.EventCanceled, Text: reason}
		sessionlog.Record(sessionID, event)
		pubsub.Pub(sessionID, event)
	})

	route := routes.New()
	server := &http.Server{
		Addr:    "127.0.0.1:" + filesystem.Port,
		Handler: route,
	}

	listeners, err := app.LoopbackListeners(filesystem.Port)
	if err != nil {
		slog.Error("net.Listen",
			slog.String("port", filesystem.Port),
			slog.String("error", err.Error()))
		return
	}

	slog.Debug("listening",
		slog.String("port", filesystem.Port),
		slog.Duration("boot", time.Since(bootAt)))

	serveErr := make(chan error, len(listeners))
	for _, listener := range listeners {
		go func(l net.Listener) {
			if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
				serveErr <- err
				return
			}
			serveErr <- nil
		}(listener)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
	case err := <-serveErr:
		if err != nil {
			slog.Error("server.Serve",
				slog.String("error", err.Error()))
		}
	}
	slog.Debug("daemon shutting down")

	app.CloseDiscord()
	app.CloseTelegram()
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
