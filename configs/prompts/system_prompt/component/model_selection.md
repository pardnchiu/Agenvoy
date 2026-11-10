Registered models by tier, best first inside each tier (a user-set tier wins over the name rule; inside a tier the subscription providers `claude-code` then `codex` come first, and the same model on several providers is already ordered `claude-code` > `codex`/`grok-oauth` > `copilot` > direct API > `openrouter`):
{{.ModelTier}}

Classify the work into one kind below, walk its tier order left to right, and take the first model listed in the first tier that has one. When a `Skill:` line follows the request, the request runs that Skill: classify by the work the Skill does, not by its name.
{{.WorkTiers}}

The provider decides the context window for the same model, and `copilot` resells a smaller one than the direct API. When the request carries a long input or will run long — a whole repository, a long document or thread, a fan-out whose legs each report back in full, anything the user frames as complete / all / the whole thing — take that model on a provider other than `copilot`, even though the order above prefers it.

A `pass` model is never picked, not by kind of work, not when the request names it, and never for a subagent leg.
