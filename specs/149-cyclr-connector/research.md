# Phase 0 Research: Cyclr Connector

Decisions that need to be locked before Phase 1 design, with alternatives considered and rejection reasons.

---

## §1. Packaging: two providers vs one-with-modules

**Decision**: Two providers — `cyclrPartner` and `cyclrAccount`.

**Rationale**:
- Partner-scope and Account-scope in Cyclr use **different OAuth token requests** (`scope=account:{API_ID}` parameter at token time) and **different request headers** (`X-Cyclr-Account` required on every Account call). They are different authentication contexts, not different views of the same auth context.
- The library's "one connection = one auth context" pattern (CLAUDE.md, BEST_PRACTICES.md §3) points directly at two providers.
- Salesforce's CRM-vs-Pardot modules share one OAuth token; Cyclr's Partner-vs-Account do not. The module abstraction is the wrong shape here.
- The downstream `fraios/saas-gateway` stores credentials per `solution_link_id`. One solution per Cyclr Account is the natural mapping; forcing both scopes into a single provider would conflate two credential tuples into one storage row.

**Alternatives considered**:
- **Option A (single provider with `scope` input)** — rejected: moves scope dispatch into the connector at request time, which the library's handler signatures don't model cleanly. Also hides from the catalog that these are two distinct auth surfaces.
- **Option C (single provider with both scopes active, namespaced object paths)** — rejected: requires the credential tuple to carry both a Partner client-credentials pair *and* an Account-scoped token, and requires path-prefix dispatch in every handler. All cost, no observable benefit.

**Source**: Spec clarification Q1 (2026-04-18 session).

---

## §2. Region handling: `{{.apiDomain}}` metadata input vs `{{.workspace}}` template

**Decision**: Cyclr region is captured as a `Metadata.Input` entry named `apiDomain` (or similar) and used in the BaseURL template.

**Rationale**:
- Cyclr's region is a transport-layer concern (which instance of Cyclr is this?) and does not affect OAuth scope. Treating it as a "workspace" in the Ampersand sense is a category error — `workspace` semantically maps to a tenant inside a SaaS, but every Cyclr region hosts many independent Partners.
- Existing workspace-templated providers (Marketo, Okta, Zuora) use `{{.workspace}}` because the URL itself is tenant-specific (`{munchkin-id}.mktorest.com`). Cyclr's URL is **instance-specific**, not tenant-specific — `api.cyclr.com` is shared across all Partners in that region.
- Using `{{.apiDomain}}` makes the intent explicit and avoids confusing future maintainers who expect `{{.workspace}}` to mean "this Partner's private URL".

**Concrete values** this `apiDomain` can take (closed set): `api.cyclr.com`, `api.us2.cyclr.com`, `api.eu.cyclr.com`, `api.cyclr.uk`, plus any operator-configured private instance.

**Alternatives considered**:
- **`{{.workspace}}` reused** — rejected: semantic mismatch; confuses catalog UIs that label the field "Workspace".
- **Hard-coded `api.cyclr.com`** — rejected: breaks every non-default region customer immediately.
- **Derived from OAuth token URL** — rejected: couples two unrelated concerns and prevents use against private instances.

---

## §3. Schema shape: static `schemas.json` vs dynamic fetch

**Decision**: Static, hand-authored `schemas.json` embedded per provider via `//go:embed`, loaded through `scrapper.NewMetadataFileManager[staticschema.FieldMetadataMapV1]` and exposed via `schema.NewOpenAPISchemaProvider`.

**Rationale**:
- Cyclr does not publish an OpenAPI spec or a discoverable metadata-introspection endpoint for its own resources (Accounts, Cycles, Templates). There is no equivalent of Salesforce's `/describe` or HubSpot's `/properties`.
- Field surface is stable and small: Account has ~8 mutable fields; Cycle has ~10 read-visible fields. Hand-authored schema is feasible and low-maintenance.
- Static schema is the reference pattern in `providers/smartleadv2/` and aligns with BEST_PRACTICES.md §10 for providers without introspection.
- Dynamic schema (fetching metadata from the provider API) adds HTTP round-trips on connector init for zero added correctness — the shape doesn't change per-Account.

**Alternatives considered**:
- **Dynamic per-object schema fetch** (`schema.NewObjectSchemaProvider`) — rejected: no upstream to fetch from.
- **Dynamic aggregate fetch** (`schema.NewAggregateSchemaProvider`) — rejected: same reason.
- **Hybrid (static base + dynamic custom fields)** as Capsule does — deferred: Cyclr has no custom-field concept at the Account or Cycle level.

---

## §4. Shared code between `cyclrpartner` and `cyclraccount`

**Decision**: No shared package. Two parallel, standalone packages.

**Rationale**:
- The two providers share: OAuth client-credentials wiring (already handled by `common.NewOAuthHTTPClient`), error body shape (~10 lines of Go), UUID regex (1 line).
- That's ≤15 lines of duplication. Pulling it into `providers/cyclrshared/` or `providers/cyclrpartner/internal/shared/` means:
  - An extra import in two files
  - A cross-package change every time either provider's shape drifts
  - A sync-time risk if upstream `amp-labs/connectors` later adds its own Cyclr provider (unlikely, but real)
- The repo's convention (every provider package is self-contained) points at no-shared.

**Alternatives considered**:
- **`providers/cyclrshared/`** — rejected: too early. If a third Cyclr-shaped provider appears, revisit.
- **Shared helpers in `common/`** — rejected: bleeds Cyclr-specific behaviour into a package shared across all connectors.

---

## §5. Pagination shape

**Decision**: Query parameters `page` (1-indexed) and `per_page`; detect end-of-pages via presence/absence of `page` query past the total, driven by response header `Total-Pages` (and/or `Total-Records`). `NextPageFunc` returns `""` when `page >= Total-Pages`.

**Rationale**:
- Cyclr API community documentation and observed responses on list endpoints (Accounts, Cycles, Templates) follow the .NET WebAPI `page` / `per_page` convention with `Total-Pages` and `Total-Records` response headers.
- Per-page defaults at 50; maximum 100 per Cyclr convention. We'll request `per_page=50` as default for MVP.
- `httpkit.HeaderLink` from Capsule's Link-header pattern does not apply here — Cyclr does not emit RFC 5988 Link headers.

**Implementation shape**:

```go
func makeNextRecordsURL(currentPage int, resp *common.JSONHTTPResponse) common.NextPageFunc {
    return func(_ *ajson.Node) (string, error) {
        total, _ := strconv.Atoi(resp.Header.Get("Total-Pages"))
        if currentPage >= total {
            return "", nil
        }
        return strconv.Itoa(currentPage + 1), nil
    }
}
```

`buildReadRequest` pulls `page` from `params.NextPage` (default `"1"` when empty) and sets `per_page=50`.

**Alternatives considered**:
- **Cursor / opaque token** — rejected: Cyclr doesn't emit one.
- **Link header parsing** — rejected: Cyclr doesn't emit it.
- **Offset/limit** — rejected: Cyclr's documented convention is page/per_page, and using offset would fight the API.

**Open confirmation**: exact header names (`Total-Pages` vs `X-Total-Pages`) to be verified on the first real-API Layer-2 test run. If different, flip a constant.

---

## §6. Error body shape

**Decision**: Single `FormatTemplate` matching .NET-style error bodies (`Message`, optional `ExceptionMessage`, optional `ModelState`), plus `statusCodeMapping` for 400 → `ErrBadRequest`, 401 → `ErrAuthentication`, 403 → `ErrForbidden`, 404 → `ErrNotFound`, 422 → `ErrBadRequest`, 429 → `ErrRetryable`/equivalent.

**Rationale**:
- Cyclr is a .NET WebAPI; observed error bodies are the standard `{ "Message": "...", "ExceptionMessage": "...", "ExceptionType": "...", "StackTrace": "..." }` or for validation errors `{ "Message": "The request is invalid.", "ModelState": { "field": ["error 1", "error 2"] } }`.
- No 200-with-error-body pattern observed (unlike Marketo). Standard HTTP status codes are used.
- `interpreter.NewFormatSwitch` matches on `MustKeys: ["Message"]` — single format is enough; `ModelState` becomes an optional field on the error descriptor.

**Implementation sketch**:

```go
var errorFormats = interpreter.NewFormatSwitch(
    interpreter.FormatTemplate{
        MustKeys: []string{"Message"},
        Template: func() interpreter.ErrorDescriptor { return &ResponseError{} },
    },
)

type ResponseError struct {
    Message          string              `json:"Message"`
    ExceptionMessage string              `json:"ExceptionMessage,omitempty"`
    ModelState       map[string][]string `json:"ModelState,omitempty"`
}

func (r ResponseError) CombineErr(base error) error {
    msg := r.Message
    if r.ExceptionMessage != "" {
        msg = fmt.Sprintf("%s (%s)", msg, r.ExceptionMessage)
    }
    if len(r.ModelState) > 0 {
        var parts []string
        for field, errs := range r.ModelState {
            parts = append(parts, fmt.Sprintf("%s: %s", field, strings.Join(errs, "; ")))
        }
        sort.Strings(parts)
        msg = fmt.Sprintf("%s [%s]", msg, strings.Join(parts, "; "))
    }
    return fmt.Errorf("%w: %s", base, msg)
}
```

**Alternatives considered**:
- **Multi-format switch** — rejected: Cyclr is consistent, no branching needed.
- **Custom HTML error handler** — rejected: Cyclr API returns JSON on 4xx/5xx; HTML is not observed.

---

## §7. Token caching strategy

**Decision**: Per-connection in-process `oauth2.ReuseTokenSource` wrapping a `clientcredentials.Config`. No cross-process cache.

**Rationale**:
- Cyclr tokens are valid for 14 days. Even at 200 concurrent `cyclrAccount` connections, each connection refreshes at most once every 14 days — a handful of token-endpoint calls per day across the whole fleet. Not worth engineering a distributed cache.
- `oauth2.ReuseTokenSource` is the idiomatic choice for Go clients against OAuth2 servers; all other OAuth2 providers in this repo use it transitively via `common.NewOAuthHTTPClient`.
- Process crashes forfeit cached tokens but this is tolerable — the gateway's process lifecycle is not measured in tokens lost.

**Alternatives considered**:
- **Disk-backed cache** — rejected: pointless at this scale, plus adds crash-consistency concerns.
- **Shared cache across connections** — rejected: different connections have different credentials and (for `cyclrAccount`) different scope parameters, so they cannot share a token source.

---

## §8. Pass-through (Proxy) support

**Decision**: Set `Support.Proxy: true` on both `cyclrPartner.go` and `cyclrAccount.go` ProviderInfo. No additional code — the `generic` package provides the implementation, wired by the downstream gateway's `factory.go`.

**Rationale**:
- BEST_PRACTICES.md §1 and CONTRIBUTING.md describe the proxy pattern: `Support.Proxy: true` in ProviderInfo, nothing else. The `generic.NewConnector` in the gateway's `internal/connector/factory.go` picks it up automatically.
- For `cyclrAccount`, the `X-Cyclr-Account` header must be added on every request. This is handled by the authenticated HTTP client construction (not by `generic` itself), meaning the same client is used for both deep calls and pass-through calls. See §9.

**Alternatives considered**:
- **Custom pass-through surface** — rejected: no need; the generic proxy already handles header injection when the client is configured correctly.

---

## §9. `X-Cyclr-Account` header injection

**Decision**: Inject `X-Cyclr-Account` at the `http.RoundTripper` layer in `cyclraccount.Connector.constructor`, wrapping the authenticated OAuth2 transport. Every outbound request from that connection automatically carries the header.

**Rationale**:
- Handling the header in a middleware transport means:
  - Both the deep Reader/Writer/Deleter calls *and* the pass-through calls get it for free
  - Handler code doesn't need to remember to attach it (no "I forgot the header" class of bug)
  - Pass-through cannot forge a different `X-Cyclr-Account` value (FR-042 satisfied) because the transport strips and re-sets it

**Implementation sketch** (for `cyclraccount/connector.go`):

```go
func constructor(base *components.Connector) (*Connector, error) {
    accountID, err := extractAccountID(base.ProviderContext) // from Metadata.Input
    if err != nil {
        return nil, err
    }
    // Wrap the existing authenticated transport.
    client := base.HTTPClient().Client
    client.Transport = &accountHeaderTransport{
        inner:     client.Transport,
        accountID: accountID,
    }
    // …rest of Reader/Writer/Deleter wiring unchanged.
}

type accountHeaderTransport struct {
    inner     http.RoundTripper
    accountID string
}

func (t *accountHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    // Clone to avoid mutating caller's request.
    req = req.Clone(req.Context())
    req.Header.Set("X-Cyclr-Account", t.accountID) // Set (not Add) — overrides any caller-provided value.
    return t.inner.RoundTrip(req)
}
```

**Alternatives considered**:
- **Add header per handler** — rejected: brittle, breaks pass-through.
- **Connection context metadata** — rejected: headers are a transport concern, not a per-call concern.

---

## §10. Object list (MVP)

**Decision**: MVP ships with these object names. Names preserve Cyclr's API path casing (camelCase).

**`cyclrPartner` objects**:

| Object | Read | Create | Update | Delete | Notes |
|---|---|---|---|---|---|
| `accounts` | ✓ | ✓ | ✓ | ✓ | Plus suspend (`/accounts/{id}/suspend`) and resume (`/accounts/{id}/resume`) via synthetic `accounts:suspend` / `accounts:resume` write paths — see contracts. |
| `templates` | ✓ | — | — | — | Read-only catalog. |
| `connectors` | ✓ | — | — | — | Read-only catalog of available Connectors. |

**`cyclrAccount` objects**:

| Object | Read | Create | Update | Delete | Notes |
|---|---|---|---|---|---|
| `cycles` | ✓ | ✓ (via `/templates/{id}/install`) | — | ✓ | Activate/deactivate are synthetic write targets — see contracts. |
| `accountConnectors` | ✓ | ✓ (via `/connectors/{id}/install`) | — | — | Installed Connector instances in this Account. No secrets exposed (FR-032). Create supports API-key / Basic / OAuth (OAuth completes "awaiting" — see FR-033). |
| `templates` | ✓ | — | — | — | Read-only view of templates visible to this Account's Partner. |
| `cycleSteps` | ✓ (by id + parent-scoped list) | — | — | — | Step introspection for installed Cycles. List via `cycles/{cycleId}/steps` pattern. |
| `cycleSteps:prerequisites` | ✓ (synthetic, by step id) | — | — | — | Diagnostic: which mappings / authentications are missing for a Step. |
| `stepParameters` | ✓ (by id + parent-scoped list `steps/{stepId}/parameters`) | — | ✓ | — | Update Step parameter mapping (MappingType + value). MCP agent-writable. |
| `stepFieldMappings` | ✓ (by id + parent-scoped list `steps/{stepId}/fieldmappings`) | — | ✓ | — | Update Step field-mapping. Structurally identical to `stepParameters`; split because Cyclr's API splits them. |

Post-MVP candidates (not in first release): `cycleRuns`, `transactions`, `accountConnectorFields`, `accountVariables`.

---

## §13. MCP-facing design — what the connector contributes

**Decision**: The connector is designed to make the downstream gateway's MCP tool generator produce well-grouped, well-described tools **without the gateway needing Cyclr-specific knowledge**. The mechanisms are all standard Ampersand surfaces.

### Facts

1. Ampersand's library has no first-class "tool group" or "progressive disclosure" concept. See `common/types.go:635-716` — `ObjectMetadata` has `DisplayName + Fields`; `FieldMetadata` has `DisplayName + ValueType + ProviderType + ReadOnly + IsCustom + IsRequired + Values + ReferenceTo`. That's the whole metadata surface.
2. The gateway (`fraios/apps/saas-gateway`) turns these into MCP tools. How it groups, describes, and discloses tools is its concern.
3. But the gateway can only generate quality tools if the connector populates metadata faithfully.

### How this connector helps the gateway

- **Object taxonomy (FR-049)**: stable, prefix/suffix-structured names so the gateway can group by convention without a per-provider mapping. The five groups (Account lifecycle, Cycle control, Cycle diagnostics, Step configuration, Connector setup, Catalog) are encoded in the names themselves.
- **Rich `FieldMetadata` (FR-045..048)**: every field has `DisplayName`, `ValueType`, `ProviderType`, `IsRequired`, `ReadOnly`; enum fields have `Values`; lookup fields have `ReferenceTo`. This is what the MCP generator uses to produce argument schemas with validation and cross-tool linking.
- **Synthetic action objects (`cycles:activate`, `accounts:suspend`, `cycleSteps:prerequisites`)**: expose verb-style tools naturally. `:suspend` / `:activate` / `:prerequisites` map cleanly to MCP tool names like `cyclr_account_suspend`, `cyclr_cycle_activate`, `cyclr_cycle_step_prerequisites`.
- **Parent-scoped list objects (`cycles/*/steps`, `steps/*/parameters`)**: let the MCP generator produce "list children of X" tools without needing special-case logic.

### Progressive disclosure — who implements what

| Concern | Implemented by | How |
|---|---|---|
| Lazy metadata loading | Gateway | Calls `ListObjectMetadata` on demand; the connector's static schema is cheap. |
| Tool-group collapsing | Gateway | Groups by object-name prefix per FR-049 taxonomy. |
| Argument schema derivation | Gateway | From `FieldMetadata` populated by this connector. |
| Top-level summary tool | Gateway | `list_providers`, `list_objects` — not connector-level. |
| Per-object docs / descriptions | **Gap** — the library has no object-level docs field | Gateway may synthesize from `DisplayName` + heuristics, or this feature may contribute a light Ampersand-upstream proposal to add `ObjectMetadata.Description`. Noted for post-MVP. |

### What the connector deliberately does NOT do

- Does not emit MCP-protocol messages directly. That's the gateway's job.
- Does not define tool groups or descriptions in its own code. Names and field metadata carry the information; the gateway composes.
- Does not duplicate Cyclr's per-method schemas for Step parameters. `MappingType` + generic `Value`/`SourceStepId`/`VariableName` union is the agent-facing abstraction; Cyclr rejects invalid combinations with 422 and we surface the `ModelState`.

### Alternatives considered

- **Embed MCP protocol in the connector** — rejected: wrong layer, would duplicate what the gateway does.
- **Static per-object tool descriptions in a metadata file** — rejected for MVP: Ampersand has no slot for it; adding upstream is out of scope for this feature.
- **Per-Connector-method Step parameter schemas** — rejected: would require dynamic introspection of every installed third-party Connector's method surface. Agents can work with the MappingType abstraction and iteratively refine.

---

## §11. Testing strategy

**Decision**: Every Layer 1 test matrix cell has a mock; Layer 2 is runnable but not mandatory for every PR.

**Layer 1 minimum per PR**:

- `providers/cyclrpartner/metadata_test.go` — lists all partner objects; asserts schema loaded correctly.
- `providers/cyclrpartner/read_test.go` — list accounts (page 1, page 2, empty page); single account; not-found; 429 with retry.
- `providers/cyclrpartner/write_test.go` — create, update, suspend, resume; validation error (422); auth error (401).
- `providers/cyclrpartner/delete_test.go` — delete success; delete with active cycles (Cyclr refusal).
- `providers/cyclraccount/{metadata,read,write,delete}_test.go` — analogous, plus `install-from-template`, `activate`, `deactivate`.

Fixtures (canned JSON responses) live in `providers/cyclr{partner,account}/test/fixtures/` and are imported as string constants by the tests.

**Layer 2 entrypoints**: `test/cyclrPartner/{metadata,read,write,delete}/main.go` + same for `cyclrAccount`. Credentials load from `~/.ampersand-creds/cyclrPartner.json` and `~/.ampersand-creds/cyclrAccount.json` respectively. The Account variant requires `accountApiId` in the creds file.

---

## §12. Open items to confirm at Layer 2

These are non-blocking for Phase 1 design but must be confirmed on the first Layer-2 run and, if different, will result in a small follow-up PR:

1. Exact pagination header names (`Total-Pages` vs `X-Total-Pages`; `Total-Records` vs `X-Total-Count`).
2. Whether `per_page` is honored with values > 50 (some .NET APIs cap silently).
3. Exact Account `suspend`/`resume` endpoint paths (documented as `/accounts/{id}/suspend` by convention — to verify).
4. Whether `DELETE /accounts/{id}` cascades or refuses when Cycles exist (spec edge case mentions both possibilities).
5. Whether templates list endpoint accepts query filter (`visibility=account` or similar) — affects whether `cyclrAccount`'s `templates` object needs a different URL.
