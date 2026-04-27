ifneq ($(filter cli run,$(MAKECMDGOALS)),)

cli:
	@go run ./cmd/app/ cli $(filter-out $@,$(MAKECMDGOALS))

run:
	@go run ./cmd/app/ run $(filter-out $@,$(MAKECMDGOALS))

%:
	@:

else

.PHONY: help build app discord add remove planner reasoning models skills test

help:
	@echo "How to use:"
	@echo "  make build              Build binary and install to /usr/local/bin/agen"
	@echo "  make app                Start unified app (TUI + Discord + REST API)"
	@echo "  make discord            Start Discord bot server (legacy)"
	@echo "  make add                Add a provider/model"
	@echo "  make remove             Remove a provider/model"
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
