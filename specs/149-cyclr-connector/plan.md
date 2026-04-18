# Implementation Plan: Cyclr Connector

**Branch**: `149-cyclr-connector` | **Date**: 2026-04-18 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/149-cyclr-connector/spec.md`

## Summary

Deliver Cyclr support to `saas-connectors` as **two independent providers** (`cyclrPartner`, `cyclrAccount`), each single-scope, consumed downstream by `fraios/apps/saas-gateway`. Authentication is OAuth 2.0 Client Credentials; transport is region-selectable (`api.cyclr.com`, `api.eu.cyclr.com`, `api.us2.cyclr.com`, `api.cyclr.uk`, private). Deep capability on day one: Account lifecycle (create/list/read/update/suspend/resume/delete) on `cyclrPartner`, Cycle lifecycle (list/read/install-from-template/activate/deactivate/delete) on `cyclrAccount`, plus proxy passthrough on both. Ships per `CONTRIBUTING.md` PR-per-capability rule.

## Technical Context

**Language/Version**: Go 1.25+ (matches upstream `amp-labs/connectors`)
**Primary Dependencies**:
- `internal/components` — base connector, reader/writer/deleter factories, endpoint registry
- `common/oauth.go` → `NewOAuthHTTPClient` with `golang.org/x/oauth2/clientcredentials` source
- `common/interpreter` — `NewFaultyResponder`, `FormatSwitch`, `DirectFaultyResponder`
- `common/urlbuilder`, `internal/jsonquery`, `internal/staticschema`
- `internal/future`, `internal/simultaneously` — bounded concurrency (no bare `go`)

**Storage**: None in-connector. Tokens live in `oauth2.ReuseTokenSource` per-connection (in-process); credentials live in cxs2 vault and are fetched fresh by the gateway per request.

**Testing**:
- Layer 1 (mandatory for PR merge): `providers/cyclrpartner/*_test.go`, `providers/cyclraccount/*_test.go` with `mockserver` + `testroutines`
- Layer 2 (real-API): `test/cyclrPartner/{metadata,read,write,delete}/main.go`, `test/cyclrAccount/{metadata,read,write,delete}/main.go` backed by `~/.ampersand-creds/cyclrPartner.json` and `~/.ampersand-creds/cyclrAccount.json`
- Layers 3–6 (gateway / e2e / cluster): owned by downstream `fraios/apps/saas-gateway`, per `DOWNSTREAM.md` testing matrix

**Target Platform**: Linux server (runs inside the gateway pod on darwin/amd64 and linux/amd64).

**Project Type**: Library (Go module consumed by `fraios/apps/saas-gateway` via `go.mod` replace directive).

**Performance Goals**:
- p95 single Account read ≤ 2s (Cyclr API + network + decode)
- p95 onboarding flow (create Account + install 1 template + activate) ≤ 10s end-to-end
- Retry budget per caller-visible request: ≤3 attempts, ≤30s wall-clock (FR-061)

**Constraints**:
- No bare `go` — enforced by `nogoroutine` linter
- BaseURL never carries API version — `/v1.0` is appended by handlers (CLAUDE.md + BEST_PRACTICES.md §16)
- Fork `main` branch-protected; merges via PR; fork stays 0 commits ahead of upstream for unchanged providers (DOWNSTREAM.md)
- Module path frozen at `github.com/amp-labs/connectors` — downstream `replace` handles fork redirection

**Scale/Scope** (MVP targets, re-evaluate after first real-API integration run):
- Up to **500** Accounts per Partner
- Up to **100** Cycles per Account
- Up to **200** concurrent `cyclrAccount` connections in the gateway
- Token refresh load negligible: one OAuth token per connection, 14-day validity

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is an **uninitialised template** in this repo — no principles have been ratified. Per speckit convention the Constitution Check is strict only when principles exist. For this feature the binding governance is the repo's existing written conventions:

- **`CLAUDE.md`** — architecture rules (component embedding, naming, concurrency)
- **`BEST_PRACTICES.md`** — concrete patterns & gotchas (BaseURL without version, preserve field names, only embed interfaces you implement, ProviderInfo flags)
- **`CONTRIBUTING.md`** — PR-per-capability ordering (proxy → metadata → read → write → delete)
- **`DOWNSTREAM.md`** — fork/sync discipline + Layer-1…Layer-6 testing matrix

**Gate checks against these** (all must hold for this plan to proceed):

| Gate | Status | Evidence |
|---|---|---|
| G1. Deep connector embeds `*components.Connector` | Pass | Both providers' `connector.go` will embed via `components.Initialize`. |
| G2. Only embed component interfaces actually implemented | Pass | Both: Reader + Writer + Deleter + SchemaProvider. No speculative Subscribe. |
| G3. BaseURL has no version suffix | Pass | BaseURL = `https://{{.apiDomain}}` (no `/v1.0`). `apiVersion = "v1.0"` lives in package const. |
| G4. Field and object names preserve Cyclr's casing | Pass | PascalCase fields (`Id`, `Name`, `Status`, `CreatedOnUtc`) preserved. No coercion. |
| G5. No bare `go` in implementation | Pass | Any fan-out (e.g., parallel metadata fetch) uses `simultaneously.DoCtx`. |
| G6. One PR per capability | Pass | Sequence below: proxy (×2) → metadata (×2) → read (×2) → write (×2) → delete (×2). |
| G7. Fork invariant respected | Pass | New proprietary providers; sole expected divergence from upstream. |
| G8. Credentials never logged (FR-062) | Pass | Client secret and bearer token excluded from all emitted strings. API-ID is identifier (spec clarification Q2) and may appear. |

No gate violations → no Complexity Tracking entries required.

## Project Structure

### Documentation (this feature)

```text
specs/149-cyclr-connector/
├── plan.md              # This file
├── research.md          # Phase 0 output — decisions + alternatives
├── data-model.md        # Phase 1 output — entities, attributes, state
├── contracts/
│   ├── cyclrPartner.md  # Partner-scope endpoints, shapes
│   └── cyclrAccount.md  # Account-scope endpoints, shapes
├── quickstart.md        # Phase 1 output — how to run each test layer
├── checklists/
│   └── requirements.md  # From /speckit-specify
├── spec.md              # From /speckit-specify + /speckit-clarify
└── tasks.md             # /speckit-tasks output (not created here)
```

### Source Code (repository root)

```text
providers/
├── cyclrPartner.go                      # ProviderInfo for Partner-scope
├── cyclrAccount.go                      # ProviderInfo for Account-scope
├── cyclrpartner/                        # Deep package for cyclrPartner
│   ├── connector.go                     # Struct + NewConnector + constructor
│   ├── handlers.go                      # build*/parse* for Read, Write, Delete
│   ├── url.go                           # URL builders (base + /v1.0 + path)
│   ├── objects.go                       # supportedObjectsByCreate/Update/Delete
│   ├── supports.go                      # supportedOperations() → EndpointRegistryInput
│   ├── errors.go                        # errorFormats + statusCodeMapping
│   ├── metadata.go                      # schema.NewOpenAPISchemaProvider over embedded schemas
│   ├── schemas.json                     # Static schemas (accounts, templates, connectors)
│   ├── params.go                        # Suspend/Resume parameter helpers
│   └── utils.go                         # Small helpers (UUID validation, etc.)
├── cyclraccount/                        # Deep package for cyclrAccount
│   ├── connector.go                     # Embeds X-Cyclr-Account injection
│   ├── handlers.go                      # build*/parse* including activate/deactivate
│   ├── url.go
│   ├── objects.go
│   ├── supports.go
│   ├── errors.go
│   ├── metadata.go
│   ├── schemas.json                     # Static schemas (cycles, accountConnectors, templates[read])
│   ├── params.go                        # ActivateParams, InstallTemplateParams
│   └── utils.go
connector/
└── new.go                               # + imports + wrapper constructors + registry entries
test/
├── cyclrPartner/
│   ├── connector.go                     # Shared harness, credscanning.LoadPath(providers.CyclrPartner)
│   ├── metadata/main.go
│   ├── read/main.go
│   ├── write/main.go
│   └── delete/main.go
└── cyclrAccount/
    ├── connector.go
    ├── metadata/main.go
    ├── read/main.go
    ├── write/main.go
    └── delete/main.go
```

**Structure Decision**: Two parallel deep-connector packages (`providers/cyclrpartner/`, `providers/cyclraccount/`) with matching test harnesses. Cross-package sharing is explicitly rejected (see Research §4) because the divergence between Partner-scope and Account-scope is behavioural and an internal shared package would cost more in indirection than it saves on ~2 duplicated helpers.

## Phase 0 — Outline & Research

See [research.md](./research.md). Decisions summary:

1. Two providers, not one with modules — locked in spec clarification Q1. Rationale in research.md §1.
2. Region as metadata input, not `{{.workspace}}` template — research §2.
3. Static schemas for MVP — Cyclr has no OpenAPI or metadata-introspection endpoint at the object level. Research §3.
4. No shared code package between the two providers — research §4.
5. Pagination: query params `page` + `per_page` with response headers `Total-Pages` / `Total-Records` — research §5.
6. Error interpretation: Cyclr is a .NET WebAPI, single JSON shape with `Message` + optional `ModelState`. Research §6.
7. Token caching: per-connection `oauth2.ReuseTokenSource` — research §7.
8. Pass-through via `Support.Proxy: true` on both ProviderInfo entries — no custom work. Research §8.

**Output**: [research.md](./research.md).

## Phase 1 — Design & Contracts

**Prerequisites**: research.md complete.

### Artifacts

- **[data-model.md](./data-model.md)** — Partner/Account/Cycle/Template/AccountConnector entities with Cyclr-native field names, types, state transitions.
- **[contracts/cyclrPartner.md](./contracts/cyclrPartner.md)** — every object's endpoint, method, headers, request shape, response shape, error shape, pagination expectation.
- **[contracts/cyclrAccount.md](./contracts/cyclrAccount.md)** — same for Account-scoped resources + activate/deactivate/install action contracts.
- **[quickstart.md](./quickstart.md)** — how to run each test layer.

### Component interface wiring

| Provider | Embeds | Static schemas | Dynamic metadata | Proxy |
|---|---|---|---|---|
| `cyclrPartner` | `components.SchemaProvider`, `components.Reader`, `components.Writer`, `components.Deleter` | `providers/cyclrpartner/schemas.json` (accounts, templates, connectors) | No | Yes |
| `cyclrAccount` | `components.SchemaProvider`, `components.Reader`, `components.Writer`, `components.Deleter` | `providers/cyclraccount/schemas.json` (cycles, accountConnectors, templates[read-only], cycleSteps, cycleSteps:prerequisites, stepParameters, stepFieldMappings) | No | Yes |

### Agent context update

`CLAUDE.md` gets a SPECKIT marker block pointing to this plan file so subsequent Claude sessions find it automatically.

### Post-design Constitution re-check

After writing data-model.md and contracts/, re-evaluate gates G1–G8. None of the Phase 1 outputs introduce new providers, concurrency patterns, or auth flows beyond this plan. Gates remain Pass.

## PR sequencing (per CONTRIBUTING.md)

Nine PRs total, in this order. Each PR is independently green under `make lint && go test ./providers/cyclr*/... && go build ./...`.

1. `feat: Add Cyclr Partner Proxy Connector` — `providers/cyclrPartner.go` only, `Support.Proxy: true`.
2. `feat: Add Cyclr Account Proxy Connector` — `providers/cyclrAccount.go` only, `Support.Proxy: true`.
3. `feat: Add Cyclr Partner Metadata Connector` — `providers/cyclrpartner/` package with SchemaProvider + schemas.json + registration.
4. `feat: Add Cyclr Account Metadata Connector` — `providers/cyclraccount/` analog.
5. `feat: Add Cyclr Partner Read Connector` — Reader wired for accounts, templates, connectors (list + by-id).
6. `feat: Add Cyclr Account Read Connector` — Reader wired for cycles, accountConnectors, templates[read-only].
7. `feat: Add Cyclr Partner Write Connector` — Writer wired for accounts (create/update/suspend/resume).
8. `feat: Add Cyclr Account Write Connector` — Writer wired for cycles (install-from-template/activate/deactivate).
9. `feat: Add Cyclr Partner + Account Delete Connectors` — Deleter on both (accounts, cycles). May combine since each side is small.

## Complexity Tracking

No Constitution Check violations → no justifications required.
