# DOWNSTREAM.md

How this fork of `amp-labs/connectors` is consumed by the `fraios/apps/saas-gateway` service.

## Why the fork exists

The upstream `amp-labs/connectors` library provides SaaS integration primitives. Ampersand's managed orchestration service (`api.withampersand.com`, `proxy.withampersand.com`, `write.withampersand.com`) is closed-source.

This fork exists so that `smartdataHQ` can:

1. Self-host orchestration (via the `fraios/apps/saas-gateway` service) — credentials never leave our infrastructure
2. Add proprietary connectors not in upstream
3. Modify connectors before upstream merges accept the change

The fork is otherwise a pure mirror of upstream. Sync flow below keeps it that way for unchanged providers.

## Getting started on a fresh checkout

Do this once per machine. Skip if `golangci-lint --version` already reports `v2.7.1`.

```bash
# macOS prerequisites (Go 1.25+, Rust for typos-cli)
brew install go rustup-init
rustup-init -y     # needed for cargo install typos-cli

# Install linters + git hooks (runs the cargo install too)
make install/dev

# Build the custom golangci-lint binary (has nogoroutine + modulelinter plugins)
make custom-gcl

# Verify
make lint
make test
```

If `make install/linters` fails on `typos-cli` and you don't need spell-checking, the lint step will still run — it just prints a "typos not found" warning.

### Provider credentials for real-API tests

Layer 2 tests (below) hit real provider APIs. They read credentials from `~/.ampersand-creds/<provider>.json` via `common/scanning/credscanning`. Credential files are never committed.

- **OAuth providers**: run `make update-creds`, which walks you through the OAuth dance and writes the JSON file.
- **API-key providers**: create the file manually. Structure varies per auth type — `scripts/oauth/README.md` documents the OAuth flow; for API keys, copy an existing file and swap fields.
- **No creds needed** for Layer 1 (unit tests).

`credscanning.LoadPath(providers.<Name>)` resolves the path from the provider constant. If you get `file not found`, confirm the provider name matches the filename exactly (lowercased, without hyphens removed — e.g., `providers.ZendeskSupport` → `zendeskSupport.json`).

## Starting a new connector

Drop this template into a fresh claude session in this repo, filling in the placeholders:

```
New connector work. Read DOWNSTREAM.md, CONTRIBUTING.md, and CLAUDE.md first.

Adding provider: <name>
  API docs: <url>
  Auth type: <oauth2_authorization_code | oauth2_client_credentials |
              api_key_header | api_key_query | bearer_token | basic_auth>
  Base URL: <https://api.example.com>    # no version suffix
  Required metadata: <workspace | tenant | none>

Scope: <proxy-only | deep R | deep R+W | deep R+W+D | deep R+W+D+Metadata>
Reference connector to mimic: providers/<airtable | anthropic | capsule | attio | salesforce>

Branch: feat/<name>-connector
Follow CONTRIBUTING.md recipes. Run make test + make lint before PR.
```

The session has everything it needs from the committed repo: authoring conventions (`CLAUDE.md`), step-by-step recipes (`CONTRIBUTING.md`), integration + testing strategy (this file), and the provider-package examples referenced above.

After the connector merges to fork `main`, bump downstream — see "Adding a connector" below.

## How the fork is wired in downstream

The fork's `go.mod` declares:

```
module github.com/amp-labs/connectors
```

Same module path as upstream. Go resolves module paths to URLs derived from the path, so a naive `go get github.com/amp-labs/connectors` pulls from `github.com/amp-labs/connectors` (upstream), **not** this fork.

Downstream wires the fork in with a `replace` directive in `fraios/apps/saas-gateway/go.mod`:

```go
replace github.com/amp-labs/connectors => github.com/smartdataHQ/saas-connectors <pseudo-version>
```

The `require` block keeps the upstream path for Dependabot/tooling compatibility; the `replace` redirects the actual fetch to this fork. Commits land in this repo; downstream bumps the pseudo-version via `go get github.com/amp-labs/connectors@latest` (still the upstream path — the replace does the redirection).

**Do not rename the fork's module path.** Every sync from upstream would conflict on `go.mod` and every internal import in ~400 files.

## Sync flow

Fork `main` is branch-protected — direct push is refused. Syncs go through PRs:

```bash
git remote add upstream https://github.com/amp-labs/connectors.git   # once
git fetch upstream

# Verify fast-forward-only (should always be the case for a mirror)
git rev-list --count origin/main..upstream/main    # commits behind
git rev-list --count upstream/main..origin/main    # commits ahead — should be 0

# Sync via PR
git checkout -b sync/upstream-$(date +%F) upstream/main
git push -u origin sync/upstream-$(date +%F)
gh pr create --base main --title "chore: sync main with amp-labs/connectors upstream"
# Merge PR → local: git checkout main && git pull --ff-only origin main
```

Invariant: **fork `main` is 0 commits ahead of upstream `main` for unchanged providers**. New proprietary connectors added in-fork are the only expected divergence. Sync PRs stay fast-forward until a proprietary commit lands.

## What downstream uses from this fork

The gateway depends on these packages:

| Package | Consumed by (gateway file) | Role |
|---|---|---|
| `connectors` (root) | every handler in `internal/handler/` | `Connector`, `ReadConnector`, `WriteConnector`, `DeleteConnector`, `BatchWriteConnector`, `ObjectMetadataConnector` interfaces |
| `connectors/common` | `internal/connector/factory.go`, all handlers | `ReadParams`, `WriteParams`, `BatchWriteParam`, `AuthenticatedHTTPClient`, `HTTPError`, `NewOAuthHTTPClient`, `NewApiKeyHeaderAuthHTTPClient`, `NewApiKeyQueryParamAuthHTTPClient`, `NewBasicAuthHTTPClient` |
| `connectors/connector` | `internal/connector/factory.go`, `internal/handler/discovery.go` | `connector.New(provider, params)` registry; `ErrInvalidProvider` sentinel |
| `connectors/generic` | `internal/connector/factory.go` | Proxy-only fallback when a provider has no deep connector |
| `connectors/providers` | everywhere | `providers.AllNames()`, `providers.ReadInfo(name)`, `providers.Provider` type |

Proto contracts on the gateway side (`specs/023-saas-gateway/contracts/gateway.proto`, `apps/saas-gateway/api/v1/gateway.proto`) map field-for-field to `common.ReadParams`/`WriteParams`/`BatchWriteParam`/etc. If these types change shape upstream, the gateway breaks at compile time — which is desirable.

## Adding a connector

Decision: proxy-only or deep?

| Need | Work required |
|---|---|
| Just API passthrough via `/v1/proxy` on the gateway — any HTTP verb, auth handled by this library | **Proxy-only** — one file: `providers/<name>.go` declaring `const <Name> Provider` + `init() { SetInfo(...) }` |
| Typed Read/Write/Delete/Batch/Metadata with pagination, field mapping, error translation | **Deep** — package at `providers/<name>/`, registered in `connector/new.go` |

### Proxy-only recipe

Add one file (reference: `providers/airtable.go`, `providers/anthropic.go`). Set `Support.Proxy: true`. Done. `generic.NewConnector` in the gateway's `internal/connector/factory.go` picks it up automatically; `ListProviders` includes it via `providers.AllNames()`.

### Deep recipe

Composition over `internal/components`. Reference small: `providers/capsule/` (R+W+D). Reference medium: `providers/attio/` (R+W+D+metadata). Reference full: `providers/salesforce/` (bulk, subscribe, metadata, apex).

Typical files in `providers/<name>/`:

- `connector.go` — `type Connector struct` composing `*components.Connector` + `components.Reader/Writer/Deleter/...`; `NewConnector(params)` + `constructor(base)`
- `objects.go` / `objectNames.go` — supported object names
- `url.go` — URL building
- `read.go` — `buildReadRequest` + `parseReadResponse`
- `write.go` — `buildWriteRequest` + `parseWriteResponse`
- `errors.go` — `errorFormats` + `statusCodeMapping` for `interpreter.NewFaultyResponder`
- `supports.go` — `supportedOperations()` for `components.NewEndpointRegistry`
- `metadata.go` + `metadata/*.json` — field schemas (only if implementing `ObjectMetadataConnector`)

Register in `connector/new.go`:

1. Add import: `"github.com/amp-labs/connectors/providers/<name>"`
2. Add entry to `connectorConstructors` map: `providers.<Name>: wrapper(new<Name>Connector),`
3. Add constructor: `func new<Name>Connector(p common.ConnectorParams) (*<name>.Connector, error) { return <name>.NewConnector(p) }`

### Validate locally

```bash
go test ./providers/<name>/... -count=1
go build ./...
make lint
```

### Land the change

PR against fork `main`. After merge, bump the gateway:

```bash
cd /path/to/fraios/apps/saas-gateway
go get github.com/amp-labs/connectors@latest   # the replace redirects to this fork
go mod tidy && go build ./... && go test ./...
```

For pre-merge gateway testing, downstream can pin to a branch:
```bash
go get github.com/amp-labs/connectors@feat/<name>-connector
```

## Testing across the stack

A new connector touches code in two repos and runs in production against three external systems (provider API, cxs2 credential source, Kafka). Don't skip layers — each catches a different class of bug.

### Layer 1 — Fork unit tests (per-connector, mocked)

```bash
cd /path/to/saas-connectors
go test -v ./providers/<name>/... -count=1
make lint
```

Fixtures in `providers/<name>/*_test.go` use mocked HTTP responses. No creds needed. **Required for PR merge into this fork.** See `CONTRIBUTING.md` "Testing your proxy connector" and `CLAUDE.md` "Testing" sections.

### Layer 2 — Fork real-API integration (credentialed)

```bash
# OAuth providers: refresh tokens first
make update-creds

# Real API calls
go run ./test/<name>/metadata
go run ./test/<name>/read
go run ./test/<name>/write
go run ./test/<name>/delete
```

Harness in `test/<name>/connector.go` loads creds via `credscanning.LoadPath(providers.<Name>)` (default path: `~/.ampersand-creds/<provider>.json`). **Proves the connector works against the live provider API in isolation — without the gateway, without cxs2.**

### Layer 3 — Gateway unit tests (after fork bump)

```bash
cd /path/to/fraios/apps/saas-gateway
go get github.com/amp-labs/connectors@latest   # replace redirects to this fork
go mod tidy
go test ./internal/... -count=1 -timeout=60s
```

Exercises handlers, cache, factory, auth, circuit breaker, rate limiter with mocked connectors. Catches interface-shape changes and auth-method-dispatch bugs. **Required after every fork bump.**

### Layer 4 — Gateway local e2e (real cxs2, real provider)

```bash
# Terminal 1: gateway against local cxs2
cd /path/to/fraios/apps/saas-gateway
export TOKEN_SECRET=$(grep TOKEN_SECRET /path/to/cxs2/.env.local | cut -d= -f2)
export FRAIOS_URL=https://local.fraios.dev:3000
export KAFKA_BROKERS=localhost:9092
go run ./cmd/server

# Terminal 2: mint JWT + probe
cd /path/to/cxs2
bun run scripts/mcp-create-test-token.ts > /tmp/jwt

curl http://localhost:8080/v1/providers/<name>
curl -X POST http://localhost:8080/v1/read \
  -H "Authorization: Bearer $(cat /tmp/jwt)" \
  -H "Content-Type: application/json" \
  -d '{"solution_link_id":"<id>","object_name":"<obj>","fields":["id","name"]}'
```

Exercises full pipeline: JWT validation → cxs2 credential fetch → vault decrypt → connector init → real provider call → audit event to Kafka → response mapping. **Catches wiring bugs that units miss — auth-method mismatch, missing metadata, error-mapping issues.**

### Layer 5 — Gateway e2e test suite (automated)

```bash
cd /path/to/fraios/apps/saas-gateway
go test ./internal/integration/ -v -count=1 -timeout=120s
```

Existing suite at `internal/integration/integration_test.go` builds the binary, starts it as a subprocess, hits real cxs2 + Airtable. Add a case for the new provider alongside existing ones. Skips gracefully if cxs2/TOKEN_SECRET unavailable.

### Layer 6 — Dev cluster deploy

```bash
cd /path/to/fraios
git commit -m "chore(saas-gateway): bump connectors for <name>"
git push     # ArgoCD auto-syncs dev overlay
# Watch: grafana.fraios.dev dashboard for request rate + errors on new provider
```

Verifies against **real provider API through real network policy**, with production-shape Prometheus metrics and Grafana dashboard. Only here will network-policy and Infisical-secrets bugs surface.

### Which layers are mandatory per change type

| Change | L1 | L2 | L3 | L4 | L5 | L6 |
|---|---|---|---|---|---|---|
| Proxy-only connector added | ✓ | — | ✓ | ✓ | — | optional |
| Deep connector added | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| New auth method type | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Upstream sync (no proprietary changes) | ✓ | — | ✓ | — | — | — |
| Gateway-only bug fix | — | — | ✓ | ✓ | ✓ | ✓ |

### Downstream constitution rule

The downstream project's `CLAUDE.md` states: *"Nothing is done before proper and complete e2e testing is done where no mocking is allowed."* Practical reading: **L4 at minimum** for any deep connector; **L5+L6** before declaring production-ready.

## Auth method extension (rare)

A new provider auth mechanism not already in the library requires:

- **This repo**: add the auth helper under `common/` (new `NewXyzAuthHTTPClient`) following existing patterns
- **Downstream** (`fraios/apps/saas-gateway`):
  - New `AuthType*` constant in `internal/credentials/client.go`
  - New `case` in `internal/connector/factory.go:createAuthClient` building the right `common.AuthenticatedHTTPClient`
  - Update `specs/023-saas-gateway/contracts/cxs2-internal-api.md` auth-method mapping table
- **cxs2** (separate repo): `solutions.authMethods[].type` validator accepting the new type

These must land together — the library change alone is not sufficient for downstream to use the new auth type.

## Cross-references inside this repo

- `CONTRIBUTING.md` — "Adding a Proxy Connector" and "Adding a Deep Connector" step-by-step, including auth-type selection
- `CLAUDE.md` — architecture, naming conventions, concurrency rules (never use bare `go`), error handling conventions
- `README.md` — library-level overview, installation, basic usage
- `Makefile` — `make test`, `make lint`, `make install/dev`, `make update-creds`, `make test-proxy`
- `.golangci.yml` — lint config; `make custom-gcl` builds the required custom golangci-lint binary
- `scripts/proxy/proxy.go` — standalone dev proxy server reference (shows auth-type dispatch patterns)
- `scripts/oauth/token.go` — OAuth credential refresh helper used by `make update-creds`

### Stale target

`Makefile` has a `connector-gen` target pointing at `scripts/connectorgen/main.go` — that directory no longer exists in upstream. The target is stale; ignore it. Use the recipes in `CONTRIBUTING.md` instead.

## Gotchas

- **Module path stays upstream.** Never edit the `module` line in this fork's `go.mod`. Every sync breaks if you do.
- **Branch protection on `main`.** Direct push refused even for fast-forward syncs. Always PR.
- **Public module proxy handles the fork.** No `GOPRIVATE` / auth needed in downstream Dockerfile — the fork is a public repo.
- **CGO is downstream's concern, not this repo's.** This library builds with pure Go; the `confluent-kafka-go` CGO requirement is in the gateway, not here.
- **Do not carry untested connectors to the downstream bump.** If a connector's `go test ./providers/<name>/...` is red or the package doesn't build, downstream `go build ./...` fails — the gateway compiles every imported package transitively through `connector/new.go`.

## Links

- Gateway service: `fraios/apps/saas-gateway/`
- Gateway spec: `fraios/specs/023-saas-gateway/`
- Proto contract: `fraios/apps/saas-gateway/api/v1/gateway.proto`
- cxs2 credential contract: `fraios/specs/023-saas-gateway/contracts/cxs2-internal-api.md`
- Upstream: https://github.com/amp-labs/connectors
- This fork: https://github.com/smartdataHQ/saas-connectors
