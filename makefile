.PHONY: help discord add remove set-planner cli run reasoning

help:
	@echo "How to use:"
	@echo "  make discord            Start Discord bot server"
	@echo "  make add                Add a provider/model"
	@echo "  make remove             Remove a provider/model"
	@echo "  make planner            Set planner model"
	@echo "  make reasoning          Set reasoning level"
	@echo "  make list               Get model list"
	@echo "  make skill-list         Get skill list"
	@echo "  make cli <input...>     Run agent (requires tool confirmation)"
	@echo "  make run <input...>     Run agent (allow all tools)"

discord:
	@go run ./cmd/server/main.go

add:
	@go run ./cmd/cli/ add

remove:
	@go run ./cmd/cli/ remove

planner:
	@go run ./cmd/cli/ planner

reasoning:
	@go run ./cmd/cli/ reasoning

list:
	@go run ./cmd/cli/ list

skill-list:
	@go run ./cmd/cli/ list skill

cli:
	@go run ./cmd/cli/ run $(filter-out $@,$(MAKECMDGOALS))

run:
	@go run ./cmd/cli/ run-allow $(filter-out $@,$(MAKECMDGOALS))

%:
	@:
