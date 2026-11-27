.PHONY: help build app dev stop update test

help:
	@echo "How to use:"
	@echo "  make build              Build binary and install to /usr/local/bin/agen"
	@echo "  make app                Attach TUI; spawn server daemon (HTTP + Discord + Telegram) if not running"
	@echo "  make dev                Same as app, but serves page/ from disk so index.html/public edits need no rebuild"
	@echo "  make stop               Stop the running server daemon"
	@echo "  make update             Update agen to the latest release (always overwrite)"
	@echo "  make test               Run all unit tests"

build:
	@git fetch --tags --force 2>/dev/null || true
	@ver=$$(git describe --tags --exact-match 2>/dev/null || echo '(dev)'); \
	go build -tags "fts5" -ldflags "-X github.com/pardnchiu/agenvoy/internal/runtime.CurrentVersion=$$ver" -o agen ./cmd/app/ && sudo mkdir -p /usr/local/bin && sudo mv agen /usr/local/bin/agen
	@rm -rf "$$HOME/.config/agenvoy/skills/.system" "$$HOME/.config/agenvoy/vendor"
	@mkdir -p "$$HOME/.config/agenvoy/skills/.system"
	@[ -d extensions/skills ] && cp -R extensions/skills/. "$$HOME/.config/agenvoy/skills/.system/" || true
	@rm -rf "$$HOME/.config/agenvoy/hooks/demo" && mkdir -p "$$HOME/.config/agenvoy/hooks/demo" && cp -R configs/hooks/demo/. "$$HOME/.config/agenvoy/hooks/demo/"
	@find "$$HOME/.config/agenvoy/skills/.system" -name __pycache__ -prune -exec rm -rf {} + 2>/dev/null || true

app:
	@$(MAKE) stop
	@$(MAKE) build
	@agen

dev:
	@$(MAKE) stop
	@$(MAKE) build
	@AGENVOY_PAGE_DIR=$(CURDIR)/page agen

stop:
	go run ./cmd/app/ stop

update:
	@go run ./cmd/app/ update

test:
	@go test -v -count=1 ./...
