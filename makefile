.PHONY: help discord add remove set-planner cli run

help:
	@echo "How to use:"
	@echo "  make discord            Start Discord bot server"
	@echo "  make add                Add a provider/model"
	@echo "  make remove             Remove a provider/model"
	@echo "  make set-planner        Set planner model"
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

cli:
	@go run ./cmd/cli/ run $(filter-out $@,$(MAKECMDGOALS))

run:
	@go run ./cmd/cli/ run-allow $(filter-out $@,$(MAKECMDGOALS))

%:
	@:
