ifneq ($(filter cli run,$(MAKECMDGOALS)),)

cli:
	@go run ./cmd/app/ run $(filter-out $@,$(MAKECMDGOALS))

run:
	@go run ./cmd/app/ run-allow $(filter-out $@,$(MAKECMDGOALS))

%:
	@:

else

.PHONY: help app discord add remove planner reasoning models skills test

help:
	@echo "How to use:"
	@echo "  make app                Start unified app (TUI + Discord + REST API)"
	@echo "  make discord            Start Discord bot server (legacy)"
	@echo "  make add                Add a provider/model"
	@echo "  make remove             Remove a provider/model"
	@echo "  make planner            Set planner model"
	@echo "  make reasoning          Set reasoning level"
	@echo "  make models             Get model list"
	@echo "  make skills             Get skill list"
	@echo "  make cli <input...>     Run agent (requires tool confirmation)"
	@echo "  make run <input...>     Run agent (allow all tools)"

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
