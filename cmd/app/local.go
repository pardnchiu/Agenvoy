package main

import (
	"log/slog"

	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"

	"github.com/pardnchiu/agenvoy/internal/agents"
	agenttypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/app"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	"github.com/pardnchiu/agenvoy/internal/runtime/torii"
	usagelog "github.com/pardnchiu/agenvoy/internal/session/usage"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func openLocal() (func(), error) {
	if err := filesystem.OpenDB(); err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Internal, "filesystem: OpenDB", "", err)
	}
	if err := historystore.New(); err != nil {
		slog.Warn("historystore.New",
			slog.String("error", err.Error()))
	}
	if err := usagelog.New(); err != nil {
		slog.Warn("usagelog.New",
			slog.String("error", err.Error()))
	}
	if !runtime.IsCurrent() {
		if err := app.SpawnDaemon(); err != nil {
			slog.Warn("daemon launch failed; running without server",
				slog.String("error", err.Error()))
		}
	}
	if err := torii.Init(filesystem.StoreDir); err != nil {
		usagelog.Close()
		historystore.Close()
		filesystem.CloseDB()
		return nil, utilsmsg.Wrap(utilsmsg.Internal, "torii: Init", "", err)
	}
	if err := go_pkg_sandbox.CheckDependence(); err != nil {
		slog.Error("sandbox.CheckDependence",
			slog.String("error", err.Error()))
	}

	registry := app.NewAgentRegistry()
	agents.Set(nil, app.SelectSummary(registry), registry, runtime.NewSkillScanner())
	agents.SetRefresher(func() (agenttypes.Agent, agenttypes.Agent, agenttypes.ModelSet) {
		registry := app.NewAgentRegistry()
		return nil, app.SelectSummary(registry), registry
	})
	agents.MarkLoaded()

	return func() {
		torii.Close()
		usagelog.Close()
		historystore.Close()
		filesystem.CloseDB()
	}, nil
}
