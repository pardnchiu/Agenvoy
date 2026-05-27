ifneq ($(filter cli run,$(MAKECMDGOALS)),)

cli:
	@go run ./cmd/app/ cli $(filter-out $@,$(MAKECMDGOALS))

run:
	@go run ./cmd/app/ run $(filter-out $@,$(MAKECMDGOALS))

%:
	@:

else

.PHONY: help build app stop update test

help:
	@echo "How to use:"
	@echo "  make build              Build binary and install to /usr/local/bin/agen"
	@echo "  make app                Attach TUI; spawn server daemon (HTTP + Discord + Telegram) if not running"
	@echo "  make stop               Stop the running server daemon"
	@echo "  make update             Update agen to the latest release (always overwrite)"
	@echo "  make test               Run provider integration tests (skips when API keys missing)"
	@echo "  make cli <input...>     Run agent (requires tool confirmation)"
	@echo "  make run <input...>     Run agent (allow all tools, no confirmation)"

build:
	@git fetch --tags --force 2>/dev/null || true
	go build -ldflags "-X github.com/pardnchiu/agenvoy/internal/runtime/tui.projectVersion=$$(git describe --tags --abbrev=0 2>/dev/null || echo dev)" -o agen ./cmd/app/ && sudo mkdir -p /usr/local/bin && sudo mv agen /usr/local/bin/agen
	@rm -rf "$$HOME/.config/agenvoy/skills/.system" "$$HOME/.config/agenvoy/tools/.system"
	@mkdir -p "$$HOME/.config/agenvoy/skills/.system" "$$HOME/.config/agenvoy/tools/.system"
	@[ -d extensions/skills ] && cp -R extensions/skills/. "$$HOME/.config/agenvoy/skills/.system/" || true
	@[ -d extensions/scripts ] && cp -R extensions/scripts/. "$$HOME/.config/agenvoy/tools/.system/" || true
	@find "$$HOME/.config/agenvoy/skills/.system" "$$HOME/.config/agenvoy/tools/.system" -name __pycache__ -prune -exec rm -rf {} + 2>/dev/null || true

app:
	@$(MAKE) stop
	@$(MAKE) build
	@agen

stop:
	go run ./cmd/app/ stop

update:
	@go run ./cmd/app/ update

test:
	@go test -v ./test/...

endif
