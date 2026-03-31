# Architecture Rules

This document captures repository-level architecture constraints.

## Layering

Preferred dependency direction:

- handler/controller -> service
- service -> repository / provider clients / internal packages
- repository -> persistence layer

## Hard rules

- Controllers/handlers should not directly access repository code for business operations.
- Service code should not return HTTP response objects.
- Provider-specific transport behavior should live in gateway/request-building code or clearly related packages.
- Prompt and harness assets should be centralized and documented, not duplicated across unrelated packages.
- Schema validation should happen at boundaries; business rules belong in service logic.
- OpenAI chat request classification may influence account selection, but the selected upstream contract must stay explicit at the gateway/request-building layer.
- Dedicated OpenAI-compatible specialized upstreams should reuse account-level `base_url` configuration instead of adding ad-hoc provider types for narrow routing cases.

## Prompt / harness architecture

Current harness structure:
- provider-family base instructions: `backend/internal/pkg/openai/instructions.txt`
- embedded wiring: `backend/internal/pkg/openai/constants.go`
- runtime bridge prompt: `backend/internal/service/prompts/codex_opencode_bridge.txt`

When extending harness behavior:
- prefer adapter layers over prompt duplication
- keep provider fingerprinting requirements in transport code/docs
- document non-obvious upstream constraints
