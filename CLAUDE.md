# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ampersand connectors — a Go library for making API calls to 100+ SaaS providers (Salesforce, HubSpot, Zendesk, etc.). Handles auth, pagination, incremental sync, and metadata retrieval. Can be used standalone or as part of the Ampersand platform.

## Build, Test, and Lint Commands

```bash
make test              # Run all tests: go test -v ./...
make test-pretty       # Tests with readable output (gotestsum)
make test-parallel     # Parallel + repeated runs for flake detection
make lint              # Run all linters (alias: make fix, make format)
make custom-gcl        # One-time: build custom golangci-lint binary (required before lint)
make install/dev       # One-time: install linters + git hooks
```

Run a single test:
```bash
go test -v -run TestName ./providers/salesforce/...
```

Run integration tests for a specific connector:
```bash
go run ./test/<provider>/metadata   # Test metadata
go run ./test/<provider>/read       # Test read
go run ./test/<provider>/write      # Test write
go run ./test/<provider>/delete     # Test delete
```

## Architecture

### Connector Types

1. **Proxy connectors** — Simple auth-and-forward proxies. Defined as a single file `providers/<provider>.go` containing `ProviderInfo` config. All share the generic proxy implementation in `generic/`.

2. **Deep connectors** — Provider-specific logic for read/write/metadata. Live in `providers/<provider>/` subdirectories. Must implement interfaces from `connectors.go`: `ReadConnector`, `WriteConnector`, `DeleteConnector`, `ObjectMetadataConnector`, etc.

### Key Directories

- `connectors.go` — All connector interface definitions (ReadConnector, WriteConnector, etc.)
- `providers/*.go` — ProviderInfo configs (one file per provider, ~329 providers)
- `providers/<name>/` — Deep connector packages (~120 providers)
- `internal/components/` — Base implementation for deep connectors. **All new deep connectors must embed `*components.Connector`** and use component interfaces (`components.Reader`, `components.Writer`, `components.SchemaProvider`, `components.Deleter`)
- `common/` — Shared utilities: HTTP clients, auth, params, metadata types, error handling
- `internal/jsonquery/` — JSON data manipulation helpers (use instead of writing custom JSON handling)
- `internal/staticschema/` — Static schema loading for connectors without metadata APIs
- `internal/datautils/` — Data utility functions
- `generic/` — Generic proxy connector implementation
- `test/` — Integration tests organized by provider
- `tools/` — CLI tools (connector generator, file converters)

### Reference Connector

Use `providers/smartleadv2/` as the reference implementation for new deep connectors. It demonstrates the component-based pattern with embedded `*components.Connector`.

### ProviderInfo and Templating

Provider configs use Go `text/template` syntax for dynamic values (e.g., `{{.workspace}}` in BaseURL). `providers/utils.go` has `ReadInfo` to resolve these templates at runtime.

## Critical Conventions

### Concurrency

**Never use the bare `go` keyword.** Use `future.Go`/`future.GoContext` (for async operations returning results) or `simultaneously.Do`/`simultaneously.DoCtx` (for parallel execution with bounded concurrency). These are in `internal/future/` and `internal/simultaneously/`.

### Naming

- Provider packages: camelCase (`salesforce`, `adobeExperience`), abbreviations capitalized (`ironcladEU`)
- Object names: preserve provider's API names exactly, including slashes (`billing/alerts`). Same object name across read/write/metadata.
- Field names: preserve provider's exact field names. Never rename.

### Read Results

- `Fields` — flattened, processed record data (remove API wrappers like `attrs:{}`, but preserve meaningful nesting like `address.city`)
- `Raw` — complete, unmodified API response. Never alter.
- Write payloads should accept the same shape as `ReadResult.Fields` (connector handles re-wrapping internally)

### Base URLs

Base URLs in ProviderInfo must NOT contain version numbers. Version is appended by the connector logic, not baked into the config.

### Deep Connector Interfaces

Only embed component interfaces you actually implement. Don't embed `components.Reader` if you haven't implemented reading — it will return errors to users at runtime.

### Error Handling

Providers returning 200 with error bodies must be converted to proper HTTP errors (see `providers/marketo/errors.go`).

### Testing

Unit tests go in `test/<provider>/<functionality>/` (e.g., `test/salesforce/read/`). Use mocked API responses. Cover pagination, empty results, error responses. A shared `connector.go` in `test/<provider>/` instantiates the connector.

### PR Conventions

- Separate PRs for each capability: proxy first, then metadata, read, write, delete
- Title format: `feat: Add <Provider> <functionality> Connector` or `[ConnectorName] Add support for <feature>`

<!-- SPECKIT START -->
Active feature plan: [specs/149-cyclr-connector/plan.md](specs/149-cyclr-connector/plan.md)
Spec: [specs/149-cyclr-connector/spec.md](specs/149-cyclr-connector/spec.md)
Research, data model, contracts, quickstart: under [specs/149-cyclr-connector/](specs/149-cyclr-connector/)
<!-- SPECKIT END -->
