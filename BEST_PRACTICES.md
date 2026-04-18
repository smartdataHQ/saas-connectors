# Ampersand Connector Best Practices

Distilled conventions for building connectors in this repo. Complements:

- `CLAUDE.md` — architecture rules, naming, concurrency
- `CONTRIBUTING.md` — step-by-step recipes
- `DOWNSTREAM.md` — how the fork is consumed, testing matrix

This document focuses on **what a new contributor gets wrong without reading existing code**. Every rule has a concrete reference.

---

## 1. Connector types: pick correctly

| Need | Choice | Work |
|---|---|---|
| Passthrough HTTP with auth handled — any verb, any path | **Proxy-only** | One file: `providers/<name>.go` |
| Typed Read / Write / Delete / Metadata with pagination, field mapping, error translation | **Deep** | Package: `providers/<name>/` + registration |

Do the proxy PR first. Merge it. Then layer on metadata → read → write → delete as **separate PRs**. This is enforced by convention and by `CONTRIBUTING.md` ordering.

**Reference tiers**:

- Minimal deep: `providers/smartleadv2/` — 5 files, static schema, no custom URL builder
- Mid-tier: `providers/capsule/` — dynamic metadata, Link-header pagination, field flattening
- Advanced: `providers/salesforce/` — multi-module (CRM + Pardot), bulk, subscribe

---

## 2. ProviderInfo (`providers/<name>.go`)

Every provider — proxy or deep — needs this file. Registered via `init()` calling `SetInfo(<Name>, &ProviderInfo{...})`.

### BaseURL rule

**BaseURL never includes the API version**. Version is appended by connector logic (`apiVersion = "v1"` in `providers/smartleadv2/connector.go:20`). Why: multi-version providers can swap versions without rewriting every handler.

### Workspace / tenant templating

Use Go `text/template` syntax in BaseURL when the URL varies per tenant:

```go
BaseURL: "https://{{.workspace}}.api.example.com"
```

Pair with `Oauth2Opts.ExplicitWorkspaceRequired: true` (or equivalent `Metadata.Input` entry) so the platform prompts for it. Resolved at runtime by `providers/utils.go` `ReadInfo`.

### AuthType dispatch cheatsheet

| Auth | Key fields | Example |
|---|---|---|
| OAuth2 authorization code (3-legged) | `AuthType: Oauth2`, `Oauth2Opts.GrantType: AuthorizationCode`, `AuthURL`, `TokenURL` | `providers/asana.go` |
| OAuth2 client credentials (2-legged) | `AuthType: Oauth2`, `GrantType: ClientCredentials`, `TokenURL` | `providers/marketo.go` |
| API key in header | `AuthType: ApiKey`, `ApiKeyOpts.AttachmentType: "header"`, `Header.Name`, optional `ValuePrefix` | `providers/monday.go` |
| API key in query | `AuthType: ApiKey`, `AttachmentType: "query"`, `QueryParamName` | grep `AttachmentType.*query` |
| Basic auth | `AuthType: Basic` | `providers/insightly.go` |

### Flags worth knowing

- `Oauth2Opts.ExplicitScopesRequired: true` — provider requires explicit scope list at token time
- `Oauth2Opts.ExplicitWorkspaceRequired: true` — pairs with `{{.workspace}}` in BaseURL
- `PostAuthInfoNeeded: true` — additional info required **after** the OAuth token is obtained (e.g., HubSpot hub selection, Calendly account). Signals the downstream platform to run a post-auth prompt.
- `Support` struct — declare capabilities honestly (`Read`, `Write`, `Delete`, `BulkWrite`, `Proxy`, `Subscribe`). Lying here breaks discovery downstream.

---

## 3. Deep connector skeleton

### Files

Keep close to this layout; deviate only with reason.

| File | Purpose |
|---|---|
| `connector.go` | `type Connector struct` embedding `*components.Connector`; `NewConnector` + `constructor` |
| `handlers.go` | `buildReadRequest`, `parseReadResponse`, `buildWriteRequest`, `parseWriteResponse`, `buildDeleteRequest`, `parseDeleteResponse` |
| `url.go` | URL-building helpers |
| `objects.go` | `supportedObjectsByCreate`, `supportedObjectsByUpdate`, `supportedObjectsByDelete` sets |
| `supports.go` | `supportedOperations()` returning `components.EndpointRegistryInput` |
| `errors.go` | `errorFormats` (FormatSwitch) + optional `statusCodeMapping` |
| `metadata.go` | Schema provider — static (embedded JSON) or dynamic (API fetch) |
| `schemas.json` or `metadata/*.json` | Static field schemas |
| `utils.go` | Helpers (singularization, extractors) |

Capsule uses this layout in full. Smartleadv2 collapses `supports.go` into `utils.go` because the surface is small — fine for tiny connectors.

### The struct

Embed `*components.Connector` and **only the component interfaces you actually implement** (`providers/smartleadv2/connector.go:33-45`):

```go
type Connector struct {
    *components.Connector
    common.RequireAuthenticatedClient

    components.SchemaProvider
    components.Reader
    components.Writer
    components.Deleter
}
```

Embedding `components.Reader` without wiring the HTTP reader causes runtime errors on the first `Read` call — don't do it speculatively.

### The constructor

Pattern (from `providers/smartleadv2/connector.go:47-128`):

```go
func NewConnector(params common.ConnectorParams) (*Connector, error) {
    return components.Initialize(providers.Smartlead, params, constructor)
}

func constructor(base *components.Connector) (*Connector, error) {
    c := &Connector{Connector: base}

    registry, err := components.NewEndpointRegistry(supportedOperations())
    if err != nil {
        return nil, err
    }

    c.SchemaProvider = schema.NewOpenAPISchemaProvider(c.ProviderContext.Module(), schemas)

    c.Reader = reader.NewHTTPReader(
        c.HTTPClient().Client, registry, c.ProviderContext.Module(),
        operations.ReadHandlers{
            BuildRequest:  c.buildReadRequest,
            ParseResponse: c.parseReadResponse,
            ErrorHandler:  errorHandler,
        },
    )
    // Writer, Deleter similarly
    return c, nil
}
```

Key points:

- `components.Initialize` does Transport wiring, panic recovery, and validation — always use it, never build the base connector by hand.
- `NewEndpointRegistry` validates glob patterns at startup; a typo fails fast, not on the first request.
- The handler factories (`reader.NewHTTPReader`, `writer.NewHTTPWriter`, `deleter.NewHTTPDeleter`) — do not implement the Reader/Writer interfaces directly.
- `c.ProviderContext.Module()` threads the current module; required for multi-module providers, harmless for single-module.

### Register in the connector registry

Every deep connector needs an entry in `connector/new.go`:

1. Import `"github.com/amp-labs/connectors/providers/<name>"`
2. Add wrapper constructor `func new<Name>Connector(p common.ConnectorParams) (*<name>.Connector, error) { return <name>.NewConnector(p) }`
3. Register `providers.<Name>: wrapper(new<Name>Connector),` in `connectorConstructors`

Downstream gateways discover connectors only through this map.

---

## 4. supportedOperations — glob registry

`supportedOperations()` returns `components.EndpointRegistryInput`. The registry gates which object names each operation accepts and precompiles glob patterns via `gobwas/glob`.

```go
func supportedOperations() components.EndpointRegistryInput {
    readSupport  := schemas.ObjectNames().GetList(common.ModuleRoot)
    writeSupport := []string{"campaigns", "email-accounts"}

    return components.EndpointRegistryInput{
        common.ModuleRoot: {
            {Endpoint: fmt.Sprintf("{%s}", strings.Join(readSupport, ",")),  Support: components.ReadSupport},
            {Endpoint: fmt.Sprintf("{%s}", strings.Join(writeSupport, ",")), Support: components.WriteSupport},
            {Endpoint: "campaigns", Support: components.DeleteSupport},
        },
    }
}
```

Patterns also match paths with slashes (`billing/*`). Multiple matches are OR'd.

---

## 5. URL building

Use `common/urlbuilder` — don't concatenate strings.

```go
u, err := urlbuilder.New(c.ProviderInfo().BaseURL, apiVersion, params.ObjectName)
u.WithQueryParam("limit", "100")
u.WithQueryParam("offset", "0")
u.WithUnencodedQueryParam("filter", "[status]=active") // preserves brackets
u.AddPath(params.RecordId)
finalURL := u.String()
```

`WithUnencodedQueryParam` is for providers with non-standard query syntax (Odata brackets, Marketo filters). Default to `WithQueryParam` — encoding is the right thing in 95% of cases.

---

## 6. Pagination

Three shapes, all returned via `NextPage` in `ReadResult`.

### Cursor / link-header (Capsule, Attio)

Parse `Link: <url>; rel="next"` from response headers. Helper: `httpkit.HeaderLink(resp, "next")`. Plug into `common.ParseResult` via a `NextPageFunc`:

```go
func makeNextRecordsURL(resp *common.JSONHTTPResponse) common.NextPageFunc {
    return func(node *ajson.Node) (string, error) {
        return httpkit.HeaderLink(resp, "next"), nil
    }
}
```

### Offset-based (Attio custom objects)

Track offset via captured closure; end pagination by detecting short page:

```go
func makeNextRecord(offset int) common.NextPageFunc {
    return func(node *ajson.Node) (string, error) {
        size := countRecords(node)
        if size == 0 || size < DefaultPageSize {
            return "", nil
        }
        return strconv.Itoa(offset + size), nil
    }
}
```

### None (Smartleadv2)

Return `"", nil`. `Done: true` falls out naturally.

---

## 7. ReadResult — Fields vs Raw

Non-obvious contract (from `CLAUDE.md`, enforced by tests):

- `Fields` — **flattened** record, API wrappers removed. Preserve meaningful nesting (`address.city`), strip mechanical wrappers (Salesforce `attributes:{}`, Capsule custom-field arrays).
- `Raw` — **complete** unmodified response object. Never rewrite.
- Write payloads accept the **same shape as Fields**; the connector re-wraps internally.

Capsule tests (`providers/capsule/read_test.go`) demonstrate the exact expectation.

Extracting records from nested JSON: `common.ExtractRecordsFromPath("data")` for the common case. For custom paths or schema-driven field names, build a `NodeRecordsFunc` using `jsonquery`:

```go
func (c *Connector) makeGetRecords(objectName string) common.NodeRecordsFunc {
    return func(node *ajson.Node) ([]*ajson.Node, error) {
        field := metadata.Schemas.LookupArrayFieldName(c.Module(), objectName)
        return jsonquery.New(node).ArrayOptional(field)
    }
}
```

**Use `internal/jsonquery` for JSON navigation.** Do not hand-roll `map[string]any` traversal — `jsonquery` handles optional fields, type assertions, and deep paths without allocating intermediate maps.

---

## 8. Write semantics

### Create vs update

Dispatch on `params.RecordId`:

```go
method := http.MethodPost
if params.RecordId != "" {
    method = http.MethodPut // or PATCH — provider-specific
}
```

### Request wrapping

If the provider nests the payload (`{"task": {...}}`), wrap in `buildWriteRequest`:

```go
wrapped := map[string]any{
    nestedWriteObject(params.ObjectName): params.RecordData,
}
body, _ := json.Marshal(wrapped)
```

Use `naming.NewSingularString(objectName)` to convert `parties` → `party`. Hardcode exceptions in a switch (Capsule does this for `projects` → `kase` — see `providers/capsule/objects.go:38-51`).

### Response: return a RecordId

Extract the new record id via `jsonquery`, handle empty-body success case:

```go
node, ok := resp.Body()
if !ok {
    return &common.WriteResult{Success: true}, nil
}
rawID, err := jsonquery.New(node).IntegerOptional("id")
// ...
return &common.WriteResult{Success: true, RecordId: strID}, nil
```

---

## 9. Error handling

### Structured error bodies (JSON)

Define error shapes with matching keys, register via `interpreter.NewFormatSwitch`:

```go
var errorFormats = interpreter.NewFormatSwitch(
    interpreter.FormatTemplate{
        MustKeys: []string{"message"},
        Template: func() interpreter.ErrorDescriptor { return &ResponseMessageError{} },
    },
    interpreter.FormatTemplate{
        MustKeys: []string{"error"},
        Template: func() interpreter.ErrorDescriptor { return &ResponseBasicError{} },
    },
)

type ResponseMessageError struct{ Message string `json:"message"` }
func (r ResponseMessageError) CombineErr(base error) error {
    if r.Message != "" { return fmt.Errorf("%w: %s", base, r.Message) }
    return base
}
```

### Status code remapping

If a provider returns 422 for what should be a 400, map it:

```go
var statusCodeMapping = map[int]error{
    http.StatusUnprocessableEntity: common.ErrBadRequest,
}
```

Wire it into the `ErrorHandler`:

```go
errorHandler := interpreter.ErrorHandler{
    JSON: interpreter.NewFaultyResponder(errorFormats, statusCodeMapping),
}.Handle
```

### HTML error pages

Some providers return HTML for 5xx. Use `DirectFaultyResponder` with goquery (pattern in `providers/smartleadv2/errors.go`).

### 200 with error body

Some providers (Marketo) return `200 OK` with an error field. Detect in `parseReadResponse` / `parseWriteResponse` and return an error — the interpreter chain never sees these, since status is 200. See `providers/marketo/errors.go`.

---

## 10. Schema / metadata

### Static schema (preferred for small surfaces)

Embed a JSON file, load once (`providers/smartleadv2/connector.go:22-31`):

```go
//go:embed schemas.json
var schemaContent []byte

var fileManager = scrapper.NewMetadataFileManager[staticschema.FieldMetadataMapV1](
    schemaContent, fileconv.NewSiblingFileLocator())

var schemas = fileManager.MustLoadSchemas()
```

Then `schema.NewOpenAPISchemaProvider(module, schemas)` as the `SchemaProvider`.

`schemas.json` shape: `modules.<id>.objects.<name>.fields` map (see `providers/smartleadv2/schemas.json`).

### Dynamic schema (fetch from API)

Use `schema.NewAggregateSchemaProvider` (one call, all objects) or `schema.NewObjectSchemaProvider` (per-object, supports `FetchModeParallel`). Implement `buildListObjectMetadataRequest` + `parseListObjectMetadataResponse`.

### Hybrid (static base + dynamic custom fields)

Capsule does this (`providers/capsule/handlers.go:20-58`): `ListObjectMetadata` starts from `metadata.Schemas.Select`, then fetches custom fields per object and calls `objectMetadata.AddFieldMetadata` for each.

---

## 11. Authentication plumbing

Do not build `http.Client` yourself. Use the wrappers in `common/`:

- `common.NewOAuthHTTPClient(ctx, opts...)` — auto-refreshes tokens (`common/oauth.go`)
- `common.NewApiKeyHeaderAuthHTTPClient(client, headerName, prefix, key)` (`common/apiKey.go`)
- `common.NewApiKeyQueryParamAuthHTTPClient(client, paramName, key)`
- `common.NewBasicAuthHTTPClient(client, username, password)` (`common/basic.go`)

These plug into `components.Initialize` via `common.ConnectorParams.AuthenticatedClient`.

For tests, credentials are resolved by `credscanning.LoadPath(providers.<Name>)` → `~/.ampersand-creds/<provider>.json`. Filename is the provider constant lowercased, **not hyphenated** (e.g., `providers.ZendeskSupport` → `zendeskSupport.json`). This trips people up.

---

## 12. Concurrency — never bare `go`

Enforced by `nogoroutine` custom linter. Use:

- `future.Go(fn) / future.GoContext(ctx, fn)` — async task returning a value + error (`internal/future/future.go`)
- `simultaneously.Do(n, fns...) / simultaneously.DoCtx(ctx, n, fns...)` — parallel fan-out with bounded concurrency (`internal/simultaneously/simultaneously.go`)

Both short-circuit on first error and propagate panics. Real examples:

- `providers/granola/notes.go` — `simultaneously.DoCtx(ctx, maxConcurrentNotesFetches, ...)`
- `providers/zoho/metadata.go` — parallel metadata fan-out

---

## 13. Testing

### Layer 1 — unit tests with `mockserver`

Place `*_test.go` next to the code (`providers/capsule/read_test.go` is the canonical example). Pattern:

```go
tests := []testroutines.Read{
    {
        Name:  "parties first page",
        Input: common.ReadParams{ObjectName: "parties", Fields: connectors.Fields("id", "type")},
        Server: mockserver.Conditional{
            Setup: mockserver.ContentJSON(),
            If:    mockcond.And{mockcond.Path("/api/v2/parties"), mockcond.QueryParam("since", "...")},
            Then:  mockserver.ResponseChainedFuncs(
                mockserver.Header("Link", `<https://...?page=2>; rel="next"`),
                mockserver.Response(http.StatusOK, responsePartiesFirstPage),
            ),
        }.Server(),
        Expected: &common.ReadResult{Rows: 2, NextPage: "...", Done: false},
    },
}
```

Cover: pagination boundaries, empty results, error responses, 200-with-error bodies if applicable.

### Layer 2 — real-API integration

Shared harness at `test/<name>/connector.go` (loads creds via `credscanning.LoadPath`). Individual entrypoints:

```bash
go run ./test/<name>/metadata
go run ./test/<name>/read
go run ./test/<name>/write
go run ./test/<name>/delete
```

Required for deep connectors per fork policy (`DOWNSTREAM.md` Layer 2).

---

## 14. Naming and field conventions

Preserve provider idioms — ignore Go-idiomatic urges to rename.

- **Package names**: camelCase with capitalized abbreviations — `salesforce`, `adobeExperience`, `ironcladEU`, `zendeskSupport`
- **Object names**: exactly as the provider's API uses them, including slashes — `billing/alerts`, not `billingAlerts`. Same object name across read, write, metadata.
- **Field names**: exact provider casing. If the API returns `First_Name`, that's the key. No renaming, no camelCase coercion.

---

## 15. Tooling

- `scripts/proxy/proxy.go` — standalone dev proxy on `:4444`. Use for manually exercising a new proxy connector before PR.
- `scripts/oauth/token.go` — OAuth flow helper. Backing the `make update-creds` target.
- `make custom-gcl` — builds the custom golangci-lint binary with `nogoroutine` + `modulelinter`. Must run once before `make lint` works.
- `make test-parallel` — flake detection via parallel repeated runs. Run before merging anything concurrency-adjacent.
- `tools/fileconv` — embedded-file locators used by `scrapper.NewMetadataFileManager`.
- **Stale**: the `connector-gen` Makefile target points at a missing script. Ignore it — use `CONTRIBUTING.md` recipes instead.

---

## 16. Gotchas

1. **BaseURL version suffix** — never include `/v1` in `ProviderInfo.BaseURL`. Handlers add it. (`CLAUDE.md`, `providers/smartleadv2/connector.go:20`)
2. **Module path frozen** — this fork's `go.mod` declares `module github.com/amp-labs/connectors`. **Never change it**; every upstream sync and every internal import would conflict. Downstream handles redirection via `replace`. (`DOWNSTREAM.md`)
3. **Don't embed unimplemented component interfaces** — runtime errors, not compile-time.
4. **Object-name filenames** (`metadata/<object>.json`) preserve slashes, which fail on some filesystems. Several connectors work around this by replacing `/` with `_`. Check the provider's examples before naming.
5. **Untested connectors break downstream** — downstream compiles every imported package transitively via `connector/new.go`. A red-tested connector in a sync blocks the gateway build. (`DOWNSTREAM.md` — "Do not carry untested connectors to the downstream bump.")
6. **Fork invariant**: main is 0 commits ahead of upstream for **unchanged** providers. Proprietary connectors are the only expected divergence. Don't patch shared code in-fork — upstream it.
7. **`go get github.com/amp-labs/connectors@latest`** in the downstream bump still works despite the fork — the `replace` directive redirects. Don't try to "fix" the require block to point at this fork.
8. **PR split discipline**: one capability per PR (proxy → metadata → read → write → delete). Title format `feat: Add <Provider> <functionality> Connector`. Bundling fails review.

---

## 17. Quick checklist — new deep connector

- [ ] Proxy PR merged first (`providers/<name>.go` with ProviderInfo, `Support.Proxy: true`)
- [ ] `providers/<name>/` package created, embedding `*components.Connector`
- [ ] `handlers.go`: `build*Request` + `parse*Response` for every operation
- [ ] `supports.go`: `supportedOperations()` with accurate glob patterns
- [ ] `errors.go`: `errorFormats` FormatSwitch + `statusCodeMapping` if needed
- [ ] `metadata.go` or embedded `schemas.json`
- [ ] Registered in `connector/new.go`
- [ ] Unit tests in `providers/<name>/*_test.go` covering pagination, empty, error cases
- [ ] Integration harness at `test/<name>/connector.go` + per-operation mains
- [ ] `go test ./providers/<name>/... -count=1` green
- [ ] `make lint` green
- [ ] Used `future.Go` / `simultaneously.Do` for any concurrency — no bare `go`
- [ ] BaseURL has no version suffix
- [ ] Field names preserved, object names preserved
- [ ] Separate PRs per capability

---

## 18. Designing for MCP / agent consumers

The Ampersand library has **no first-class concept** of tool groups, tool descriptions, or progressive disclosure. Tool ergonomics live downstream — in whatever MCP server (e.g., `fraios/apps/saas-gateway`) wraps this library. That downstream layer can only produce high-quality agent-facing tools if the connector feeds it with the right primitives.

What the library **does** give you that matters for MCP:

| Primitive | Defined in | MCP consumer uses it for |
|---|---|---|
| `ObjectMetadata.DisplayName` | `common/types.go:637` | Tool title fragments (e.g., `"Cyclr Account"`). |
| `FieldMetadata.DisplayName` | `common/types.go:672` | Argument labels in tool schemas. |
| `FieldMetadata.ValueType` + `ProviderType` | `common/types.go:675–679` | JSON-schema type coercion + provider-specific nuance. |
| `FieldMetadata.IsRequired` | `common/types.go:691` | Marking arguments required vs optional in tool schemas. |
| `FieldMetadata.ReadOnly` | `common/types.go:682` | Hiding fields from update tools. |
| `FieldMetadata.Values` (enum) | `common/types.go:695` | JSON-schema `enum`; drop-down options in agent UIs. |
| `FieldMetadata.ReferenceTo` | `common/types.go:699` | Cross-tool linking — "this Id points at that other object's list tool". |
| Slash-containing object names | CLAUDE.md §Naming | Tool grouping by prefix (e.g., `cycles/*/steps`, `steps/*/parameters`). |
| Suffix-style synthetic objects (`:activate`, `:suspend`) | Convention (see Cyclr spec) | Verb-style action tools (`account_suspend`, `cycle_activate`). |

### Disciplines that produce agent-friendly tools

When you're building a connector whose consumers include MCP agents, adopt these:

1. **Fill every `FieldMetadata` slot that applies.** Blank `DisplayName`, missing `Values` on enums, unset `IsRequired`, nil `ReferenceTo` on lookup fields — each gap degrades a generated tool. The worst-offender cases produce tools whose arguments have no labels, no constraints, and no cross-references. Agents struggle with those tools disproportionately because they lean on metadata more heavily than humans.

2. **Choose a stable, hierarchical object-name taxonomy.** Group related operations under a shared prefix (`cycles`, `cycles:activate`, `cycles:deactivate`, `cycles/*/steps`). Downstream MCP generators can group tools by common prefix without needing a per-connector mapping. Renaming a path is a breaking change for every consuming agent — treat it as such.

3. **Use synthetic `:action` suffixes for verb-style operations** rather than inventing custom RPC-style object names. `accounts:suspend` reads unambiguously as "act on an Account with the suspend verb" and maps to a clean MCP tool name. Contrast `suspendAccount` which pollutes the object space with things that aren't objects.

4. **Prefer parent-scoped lists to flat lists with filter parameters.** `cycles/{cycleId}/steps` beats `steps?cycleId=...` — the path encodes the relationship, the MCP generator exposes it as a child-of tool, and the agent doesn't have to guess which filter key pairs with which parent.

5. **Don't type what you can't predict.** When an object's exact schema varies per-instance (e.g., Cyclr Step parameters vary per Connector method), expose a generic abstraction (`MappingType` + `Value` union) and let the downstream API validate. A partially-typed schema that breaks in unpredictable ways is worse for agents than a small, predictable abstraction.

6. **Be explicit about secret boundaries at the metadata level.** Mark credential-adjacent fields `ReadOnly: true` and consider stripping them from `Fields` in `parseReadResponse` while preserving `Raw`. MCP surfaces that log tool calls (most of them) must never see `access_token`, `password`, etc. Metadata annotations help the generator know what to redact.

7. **Populate `ReferenceTo` on every lookup field.** When a Cycle references a Template id, a Step references a Connector id, an AccountConnector references a Connector id — `ReferenceTo: []string{"templates"}` etc. An MCP generator can use this to produce "resolve this id" subtools or to link tools visually. Without it, agents get an opaque UUID with no recovery path.

### What is NOT the connector's job

- Emitting MCP-protocol messages. That's the gateway.
- Defining tool groups, descriptions, or disclosure order in Go code. Names and metadata carry the information.
- Duplicating provider-specific schemas where the provider already enforces. If Cyclr rejects an invalid mapping with 422, surface the `ModelState` — don't client-side-validate the full provider method surface.

### Gap in the library worth noting

`ObjectMetadata` has no `Description` or `DocsURL` field — only `DisplayName`. For rich per-object tool descriptions, the downstream MCP generator has to synthesize from `DisplayName` + heuristics, or the connector family has to agree on a convention (e.g., an embedded markdown file per object). A future upstream proposal to add `ObjectMetadata.Description` would pay for itself across every MCP-consumed connector.

---

## 19. Further reading (in-repo)

- `internal/components/connector.go` — base connector lifecycle
- `internal/components/operations/` — `ReadHandlers`, `WriteHandlers`, `DeleteHandlers`, `SingleObjectMetadataHandlers` shapes
- `common/interpreter/` — error translation chain
- `common/urlbuilder/` — URL builder
- `common/scanning/credscanning/` — credential file loader for tests
- `common/parse.go` — `ParseResult`, `NextPageFunc`, `MakeMarshaledDataFunc`
- `internal/jsonquery/` — JSON extraction helpers (preferred over hand-rolled traversal)
- `internal/staticschema/` — static schema types
- `providers/utils.go` — `ReadInfo` template resolution
