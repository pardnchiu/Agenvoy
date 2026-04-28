ifneq ($(filter cli run switch new,$(MAKECMDGOALS)),)

cli:
	@go run ./cmd/app/ cli $(filter-out $@,$(MAKECMDGOALS))

run:
	@go run ./cmd/app/ run $(filter-out $@,$(MAKECMDGOALS))

switch:
	@go run ./cmd/app/ switch $(filter-out $@,$(MAKECMDGOALS))

new:
	@go run ./cmd/app/ new $(filter-out $@,$(MAKECMDGOALS))

%:
	@:

else

.PHONY: help build app discord add remove config planner reasoning models skills test

help:
	@echo "How to use:"
	@echo "  make build              Build binary and install to /usr/local/bin/agen"
	@echo "  make app                Start unified app (TUI + Discord + REST API)"
	@echo "  make discord            Start Discord bot server (legacy)"
	@echo "  make add                Add a provider/model"
	@echo "  make remove             Remove a provider/model"
	@echo "  make config             Edit current CLI session bot.md in \$$EDITOR"
	@echo "  make new [name]         Start a new CLI session (optional bot.md name) and switch primary pointer"
	@echo "  make switch <name>      Switch primary pointer to the cli- session whose bot.md name matches"
	@echo "  make planner            Set planner model"
	@echo "  make reasoning          Set reasoning level"
	@echo "  make models             Get model list"
	@echo "  make skills             Get skill list"
	@echo "  make cli <input...>     Run agent (requires tool confirmation)"
	@echo "  make run <input...>     Run agent (allow all tools, no confirmation)"

build:
	@git fetch --tags --force 2>/dev/null || true
	go build -ldflags "-X github.com/pardnchiu/agenvoy/internal/tui.projectVersion=$$(git describe --tags --abbrev=0 2>/dev/null || echo dev)" -o agen ./cmd/app/ && sudo mv agen /usr/local/bin/agen

app:
	go run ./cmd/app/

discord:
	go run ./cmd/server/main.go

add:
	go run ./cmd/app/ add

remove:
	go run ./cmd/app/ remove

config:
	go run ./cmd/app/ config

planner:
	go run ./cmd/app/ planner

reasoning:
	go run ./cmd/app/ reasoning

models:
	go run ./cmd/app/ list

skills:
	go run ./cmd/app/ list skill

test:
	go test ./test/... -v -timeout 60s

endif
