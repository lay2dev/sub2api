# Product Rules

This document captures product-level rules for `sub2api`.

## Product purpose

`sub2api` is an API gateway for upstream AI providers. It is responsible for:
- account selection
- request forwarding
- authentication mediation
- quota and billing enforcement
- model and provider compatibility handling

## Product-level rules

- Gateway compatibility behavior must be explicit and auditable.
- Provider integrations must prefer configuration-driven compatibility before introducing full new platform types.
- User-visible behavior changes require corresponding docs updates.
- Upstream-specific request requirements must be documented when they affect successful routing.
- Validation should prove both normal and failure-path behavior for major gateway changes.

## For coding-agent upstreams

When integrating coding-agent providers:
- document required headers and auth modes
- document streaming format assumptions
- keep request-shaping logic close to gateway code
- avoid relying on hidden prompt hacks as the primary transport fix
