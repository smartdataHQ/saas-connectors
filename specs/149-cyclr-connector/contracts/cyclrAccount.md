# Contract: `cyclrAccount`

Account-scope provider. Credentials: OAuth 2.0 Client Credentials **with `scope=account:{API_ID}` in token request**. `X-Cyclr-Account: {API_ID}` header injected on every outbound request by the connector's transport middleware (research §9). Base URL resolved from the `apiDomain` metadata input.

---

## ProviderInfo (`providers/cyclrAccount.go`)

```go
const CyclrAccount Provider = "cyclrAccount"

func init() {
    SetInfo(CyclrAccount, ProviderInfo{
        DisplayName: "Cyclr (Account)",
        AuthType:    Oauth2,
        BaseURL:     "https://{{.apiDomain}}",
        Oauth2Opts: &Oauth2Opts{
            GrantType:                 ClientCredentials,
            TokenURL:                  "https://{{.apiDomain}}/oauth/token",
            ExplicitScopesRequired:    true, // scope=account:{API_ID} is required
            ExplicitWorkspaceRequired: false,
        },
        Support: Support{
            Proxy: true,
            Read:  true,
            Write: true,
            Delete: true,
            Subscribe: false,
            BulkWrite: BulkWriteSupport{},
        },
        Metadata: &ProviderMetadata{
            Input: []MetadataItemInput{
                {
                    Name:        "apiDomain",
                    DisplayName: "Cyclr API Domain",
                    DocsURL:     "https://community.cyclr.com/user-documentation/api/introduction-to-the-cyclr-api",
                },
                {
                    Name:        "accountApiId",
                    DisplayName: "Cyclr Account API ID",
                    DocsURL:     "https://community.cyclr.com/user-documentation/api/authorize-account-api-calls",
                },
            },
        },
    })
}
```

---

## Authenticated HTTP client

`clientcredentials.Config`:

- `ClientID`, `ClientSecret` — from connection credentials.
- `TokenURL` — resolved from `apiDomain`.
- `EndpointParams` — `scope=account:{accountApiId}`.
- `AuthStyle` — `oauth2.AuthStyleInParams`.

**Middleware transport** wraps the OAuth2 client and attaches `X-Cyclr-Account: {accountApiId}` to every outbound request. Prevents callers from overriding via pass-through (FR-042).

---

## Object: `cycles`

### List — `Read` with empty `RecordId`

- Method: `GET`
- URL: `/v1.0/cycles?page={page}&per_page=50`
- Headers: `Authorization: Bearer {token}`, `X-Cyclr-Account: {accountApiId}` (auto-injected).
- Pagination: as per research §5.
- `ReadResult.Fields` per row: `{Id, Name, Status, TemplateId, Interval, StartTime, RunOnce, Connectors, ErrorCount, WarningCount, CreatedOnUtc}`.

### Get single — `Read` with `RecordId`

- Method: `GET`
- URL: `/v1.0/cycles/{RecordId}`

### Install from template — synthetic write via `ObjectName: "cycles"` with an auxiliary `TemplateId` in the payload

- Method: `POST`
- URL: `/v1.0/templates/{TemplateId}/install`
- Body: empty
- Payload shape accepted by `buildWriteRequest`:
  ```json
  {
    "TemplateId": "7ad2265e-2ff0-477b-b913-cae1dfde2ea8"
  }
  ```
- Behaviour:
  - `params.RecordData["TemplateId"]` is extracted and placed in the URL path.
  - No other fields are read from `RecordData` for this call.
  - Response is the full Cycle object; `WriteResult.RecordId` = response `Id`.
- **Rationale for this shape**: Cyclr's install endpoint is `POST /v1.0/templates/{TemplateId}/install` with an empty body. Modelling this as "write to the `cycles` object with a `TemplateId` field" keeps the caller's mental model aligned with "I want to create a Cycle from a template", while the connector handles the URL gymnastics internally.

### Activate — synthetic write via `ObjectName: "cycles:activate"`

- Method: `PUT`
- URL: `/v1.0/cycles/{RecordId}/activate`
- Body:
  ```json
  {
    "StartTime": "2026-04-18T00:00:00Z",
    "Interval": 60,
    "RunOnce": false
  }
  ```
- `Interval` must be one of: `1, 5, 15, 30, 60, 120, 180, 240, 360, 480, 720, 1440, 10080`.
- `StartTime` is optional.
- `RunOnce` is optional (default false).

### Deactivate — synthetic write via `ObjectName: "cycles:deactivate"`

- Method: `PUT`
- URL: `/v1.0/cycles/{RecordId}/deactivate`
- Body: empty

### Delete — `Delete` with `RecordId`

- Method: `DELETE`
- URL: `/v1.0/cycles/{RecordId}`

---

## Object: `accountConnectors` (read + create)

### List

- Method: `GET`
- URL: `/v1.0/account/connectors?page={page}&per_page=50` *(exact path confirmed at Layer-2)*

### Get single

- Method: `GET`
- URL: `/v1.0/account/connectors/{RecordId}`

### Install (create) — `Write` with empty `RecordId`

- Method: `POST`
- URL: `/v1.0/connectors/{ConnectorId}/install`
- Headers: `Authorization: Bearer {token}`, `X-Cyclr-Account: {accountApiId}` (auto-injected), `Content-Type: application/json`
- Body:
  ```json
  {
    "Name": "Display name for this installation",
    "Description": "Optional",
    "AuthValue": "plain-text-key | base64(user:pass) | omit for OAuth"
  }
  ```
- Payload shape accepted by `buildWriteRequest`:
  ```json
  {
    "ConnectorId": "<catalog-connector-uuid>",
    "Name": "...",
    "Description": "...",
    "AuthValue": "..."
  }
  ```
- Behaviour:
  - `params.RecordData["ConnectorId"]` is extracted and placed in the URL path; it is NOT forwarded in the body.
  - `Name`, `Description`, `AuthValue` are forwarded in the body.
  - Response: the new AccountConnector (`Id`, `ConnectorId`, `Name`, `AuthenticationState`, `CreatedOnUtc`).
  - `WriteResult.RecordId` = response `Id`.
- **Secret hygiene on create**: the `AuthValue` field is never logged, echoed in errors, or included in telemetry (FR-034). `buildWriteRequest` MUST take care not to include `RecordData` in any error-context surface.
- **OAuth Connectors**: for OAuth-typed catalog Connectors, the caller omits `AuthValue`. The install call succeeds with `AuthenticationState = "AwaitingAuthentication"`. Completing the OAuth flow (browser redirect via Cyclr's sign-in-token endpoint) is outside this connector's scope — use pass-through or the caller's UI layer.

Update, delete, and authorisation changes on AccountConnectors are NOT part of MVP's typed surface; fall back to pass-through.

**Secret hygiene on read**: if Cyclr returns any fields resembling stored credentials (`AccessToken`, `ApiKey`, `Password`, etc.) in read responses, `parseReadResponse` strips them from `Fields` before surfacing the row. `Raw` is left untouched per FR-051, but a `// TODO(security)` is filed upstream to Cyclr if observed.

---

## Object: `cycleSteps` (read-only)

### Get single by step id — `Read` with `RecordId`

- Method: `GET`
- URL: `/v1.0/steps/{RecordId}`
- Response: single Step object per `data-model.md` §CycleStep.

### List steps for a Cycle — parent-scoped path

- Method: `GET`
- URL: `/v1.0/cycles/{CycleId}/steps?page={page}&per_page=50`
- Typed surface options (decided at implementation time, revisited at Layer-2):
  1. **Slash-containing object name**: `cycles/{cycleId}/steps` — caller provides the literal cycleId in the object name. The glob pattern `cycles/*/steps` registers the capability; `buildReadRequest` parses the cycleId out of `params.ObjectName`. This pattern is explicitly supported by this library (CLAUDE.md §Naming; BEST_PRACTICES.md §14).
  2. **Pass-through deferred**: if (1) proves ergonomically unpleasant at Layer-2, list-by-cycle falls back to pass-through while by-id typed read (above) remains typed.
- MVP SHOULD ship option 1 unless a concrete ergonomic blocker surfaces.

### Secret hygiene

Step responses may include mapped parameter/field values. If any value field's name matches a credential-shaped heuristic (`AccessToken`, `ApiKey`, `Password`, `Bearer`, `Secret`, `Token`), it is stripped from `Fields` before return. `Raw` is preserved (FR-028, FR-051).

---

## Object: `stepParameters` (read + write)

### List parameters for a Step — parent-scoped path

- Method: `GET`
- URL: `/v1.0/steps/{stepId}/parameters?page={page}&per_page=50`
- Object name: `steps/{stepId}/parameters` (glob registration: `steps/*/parameters`)
- `parseReadResponse`: apply credential-stripping heuristic to the `Value` field before populating `Fields`.

### Get single parameter

- Method: `GET`
- URL: `/v1.0/steps/{stepId}/parameters/{parameterId}`
- Addressing: object name `stepParameters`, `RecordId = parameterId`, with `StepId` provided via `RecordData.StepId` on the read call. Layer-2 confirmation: if `common.ReadParams` doesn't support passing auxiliary IDs on read, fall back to compound `RecordId = "{stepId}/{parameterId}"` with `buildReadRequest` parsing the slash.

### Update parameter mapping — `Write`

- Method: `PUT`
- URL: `/v1.0/steps/{stepId}/parameters/{parameterId}`
- Headers: `Authorization: Bearer {token}`, `X-Cyclr-Account: {accountApiId}` (auto-injected), `Content-Type: application/json`
- Payload shape accepted by `buildWriteRequest`:
  ```json
  {
    "StepId": "step-uuid",
    "MappingType": "StaticValue | ValueList | StepOutput | AccountVariable | <other>",
    "Value": "...",
    "SourceStepId": "upstream-step-uuid",
    "SourceFieldName": "upstream-field-name",
    "VariableName": "account-variable-name"
  }
  ```
  `RecordId = parameterId`. `StepId` is extracted and placed in the URL path; it is NOT forwarded in the body to Cyclr. Body sent to Cyclr is the mapping shape only:
  ```json
  { "MappingType": "StaticValue", "Value": "MyStaticValue" }
  ```
  (or the appropriate shape for the chosen `MappingType`).
- Behaviour:
  - Unknown `MappingType` values are forwarded uninterpreted (FR-037 — forward-compat).
  - Response: the updated parameter object; `WriteResult.RecordId` = the parameter's `Id`.
  - On 422 validation failure (e.g., invalid `Value` for a `ValueList` parameter), Cyclr's `ModelState` is surfaced intact.
- Secret hygiene: the submitted `Value`, `VariableName`, etc. MUST NOT appear in error messages, log lines, or telemetry (FR-039). `buildWriteRequest` constructs the outbound request without placing `RecordData` into any surface accessible to the error-interpreter's error wrapping.

---

## Object: `stepFieldMappings` (read + write)

Structurally identical to `stepParameters` (same shape, same `MappingType` set, same secret hygiene) but addressed through `/v1.0/steps/{stepId}/fieldmappings/{fieldId}`. All the bullets above apply by substitution of `parameters` → `fieldmappings`. The split exists because Cyclr's API splits them; an MCP generator downstream may choose to present both as a unified "Step inputs" tool group with a `kind` discriminator.

`supportedOperations()` registers both — see the updated registry block below.

---

## Synthetic read: `cycleSteps:prerequisites`

### Request

- Method: `GET`
- URL: `/v1.0/steps/{RecordId}/prerequisites`
- `RecordId` carries the Step identifier.
- Routing: `buildReadRequest` inspects `ObjectName` for the `:prerequisites` suffix, strips it, and routes to the prerequisites URL.

### Response

Shape is Cyclr-defined (an array of prerequisite descriptors). Preserved verbatim in `Raw`; flattened per the standard `ParseResult` into `Fields`. Exact field names confirmed at Layer-2.

No pagination.

---

## Object: `templates` (read-only view)

Same shape as `cyclrPartner`'s `templates` read surface, but scoped to the templates visible to this Account's Partner. If Cyclr exposes a different path for the Account-scoped view (e.g., `/v1.0/account/templates`), it is used here; otherwise the same `/v1.0/templates` path is reused with the Account header applying implicit scope.

---

## `supportedOperations()` shape

```go
func supportedOperations() components.EndpointRegistryInput {
    return components.EndpointRegistryInput{
        common.ModuleRoot: {
            // Reads
            {Endpoint: "{cycles,accountConnectors,templates,cycleSteps,cycleSteps:prerequisites,stepParameters,stepFieldMappings}", Support: components.ReadSupport},
            // Parent-scoped lists (globs)
            {Endpoint: "cycles/*/steps", Support: components.ReadSupport},
            {Endpoint: "steps/*/parameters", Support: components.ReadSupport},
            {Endpoint: "steps/*/fieldmappings", Support: components.ReadSupport},
            // Writes
            {Endpoint: "{cycles,cycles:activate,cycles:deactivate,accountConnectors,stepParameters,stepFieldMappings}", Support: components.WriteSupport},
            // Deletes
            {Endpoint: "cycles", Support: components.DeleteSupport},
        },
    }
}
```

---

## Error body

Same shape and wiring as `cyclrPartner` (research §6). One code path in `errors.go` can be lifted between the two packages if duplication becomes unpleasant, but research §4 rejected a shared package for MVP.

Additional `statusCodeMapping` consideration: a request with the wrong scope (Partner-scoped credential trying to hit an Account endpoint, or vice versa) typically returns 401/403 from Cyclr. The connector translates this into a clear scope-mismatch error (FR-005) by checking for a specific substring in the `Message` field of the error body; exact matching string confirmed at Layer-2.

---

## Proxy

`Support.Proxy: true`. Because the authenticated client carries the `X-Cyclr-Account` transport middleware, pass-through calls automatically get the correct Account context (FR-042). Any attempt by a caller to set `X-Cyclr-Account` themselves is silently overwritten by the middleware — Account scope is bound at connection creation and cannot be dynamic.

---

## Metadata

Static `providers/cyclraccount/schemas.json` embedded via `//go:embed`, exposed through `schema.NewOpenAPISchemaProvider`. Covers `cycles`, `accountConnectors`, `templates`, `cycleSteps`. Synthetic objects (`cycles:activate`, `cycles:deactivate`, `cycleSteps:prerequisites`, `cycles/*/steps`) share schema with their base object where applicable; the prerequisites synthetic surfaces a small standalone schema for the diagnostic shape.
