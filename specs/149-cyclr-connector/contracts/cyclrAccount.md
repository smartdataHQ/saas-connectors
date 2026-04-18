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

## Object: `accountConnectors` (read-only)

### List

- Method: `GET`
- URL: `/v1.0/account/connectors?page={page}&per_page=50` *(exact path confirmed at Layer-2)*

### Get single

- Method: `GET`
- URL: `/v1.0/account/connectors/{RecordId}`

**Secret hygiene**: if Cyclr returns any fields resembling stored credentials (`AccessToken`, `ApiKey`, `Password`, etc.) in the response, `parseReadResponse` strips them from `Fields` before surfacing the row. `Raw` is left untouched per FR-051, but a `// TODO(security)` is filed upstream to Cyclr if observed.

---

## Object: `templates` (read-only view)

Same shape as `cyclrPartner`'s `templates` read surface, but scoped to the templates visible to this Account's Partner. If Cyclr exposes a different path for the Account-scoped view (e.g., `/v1.0/account/templates`), it is used here; otherwise the same `/v1.0/templates` path is reused with the Account header applying implicit scope.

---

## `supportedOperations()` shape

```go
func supportedOperations() components.EndpointRegistryInput {
    return components.EndpointRegistryInput{
        common.ModuleRoot: {
            {Endpoint: "{cycles,accountConnectors,templates}", Support: components.ReadSupport},
            {Endpoint: "{cycles,cycles:activate,cycles:deactivate}", Support: components.WriteSupport},
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

Static `providers/cyclraccount/schemas.json` embedded via `//go:embed`, exposed through `schema.NewOpenAPISchemaProvider`. Covers `cycles`, `accountConnectors`, `templates`.
