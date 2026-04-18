# Contract: `cyclrPartner`

Partner-scope provider. Credentials: OAuth 2.0 Client Credentials. No `X-Cyclr-Account` header on any call. Base URL resolved from the `apiDomain` metadata input.

---

## ProviderInfo (`providers/cyclrPartner.go`)

```go
const CyclrPartner Provider = "cyclrPartner"

func init() {
    SetInfo(CyclrPartner, ProviderInfo{
        DisplayName: "Cyclr (Partner)",
        AuthType:    Oauth2,
        BaseURL:     "https://{{.apiDomain}}",
        Oauth2Opts: &Oauth2Opts{
            GrantType:                 ClientCredentials,
            TokenURL:                  "https://{{.apiDomain}}/oauth/token",
            ExplicitScopesRequired:    false,
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
            },
        },
    })
}
```

---

## Authenticated HTTP client

`common.NewOAuthHTTPClient` with `clientcredentials.Config`:

- `ClientID`, `ClientSecret` — from connection credentials.
- `TokenURL` — resolved from `apiDomain`.
- `EndpointParams` — empty (no `scope` parameter on Partner tokens).
- `AuthStyle` — `oauth2.AuthStyleInParams` (Cyclr expects form-encoded body, not Basic auth header).

No middleware transport beyond the OAuth one.

---

## Object: `accounts`

### List — `Read` with empty `RecordId`

- Method: `GET`
- URL: `https://{{.apiDomain}}/v1.0/accounts?page={page}&per_page=50`
- Headers: `Authorization: Bearer {token}`
- Response body: array of Account objects (exact wrapping TBD at Layer-2).
- Pagination: `page` 1-indexed; next page derived from `Total-Pages` header.
- `ReadResult.Fields` per row: all Account attributes from data-model.
- `ReadResult.Raw`: unmodified response element.

### Get single — `Read` with `RecordId`

- Method: `GET`
- URL: `/v1.0/accounts/{RecordId}`
- Response body: single Account object.

### Create — `Write` with empty `RecordId`

- Method: `POST`
- URL: `/v1.0/accounts`
- Body: `{ "Name": "...", "Description": "...", "Timezone": "...", "StepDataSuccessRetentionHours": N, "StepDataErroredRetentionHours": N, "TransactionErrorWebhookEnabled": bool, "TransactionErrorWebhookUrl": "...", "TransactionErrorWebhookIncludeWarnings": bool }`
- Response: full Account including `Id` and `CreatedOnUtc`.
- `WriteResult.RecordId`: `Id`.

### Update — `Write` with `RecordId`

- Method: `PUT`
- URL: `/v1.0/accounts/{RecordId}`
- Body: partial Account object (same shape as Create, minus `Id`).
- Response: updated Account.

### Suspend — synthetic write via `ObjectName: "accounts:suspend"`

- Method: `POST`
- URL: `/v1.0/accounts/{RecordId}/suspend`
- Body: empty.
- `WriteResult.RecordId`: the same `RecordId` echoed back.
- **Routing note**: the connector's `buildWriteRequest` inspects `ObjectName` for the `":suspend"` suffix, strips it, and routes to the suspend URL. The object-name suffix is the lowest-friction way to surface an action as a typed write without inventing a new component interface.

### Resume — synthetic write via `ObjectName: "accounts:resume"`

- Method: `POST`
- URL: `/v1.0/accounts/{RecordId}/resume`
- Body: empty.

### Delete — `Delete` with `RecordId`

- Method: `DELETE`
- URL: `/v1.0/accounts/{RecordId}`
- Response: 204 No Content on success; 4xx with a JSON error body on refusal.

---

## Object: `templates` (read-only)

### List

- Method: `GET`
- URL: `/v1.0/templates?page={page}&per_page=50`

### Get single

- Method: `GET`
- URL: `/v1.0/templates/{RecordId}`

Write/Delete are unsupported; `supportedOperations()` must NOT declare them for this object — any attempt returns Cyclr's 405/404 translated to a typed error.

---

## Object: `connectors` (read-only)

### List

- Method: `GET`
- URL: `/v1.0/connectors?page={page}&per_page=50`

### Get single

- Method: `GET`
- URL: `/v1.0/connectors/{RecordId}`

---

## `supportedOperations()` shape

```go
func supportedOperations() components.EndpointRegistryInput {
    return components.EndpointRegistryInput{
        common.ModuleRoot: {
            {Endpoint: "{accounts,templates,connectors}", Support: components.ReadSupport},
            {Endpoint: "{accounts,accounts:suspend,accounts:resume}", Support: components.WriteSupport},
            {Endpoint: "accounts", Support: components.DeleteSupport},
        },
    }
}
```

---

## Error body

Single JSON format matching `{ "Message": "...", "ExceptionMessage": "...", "ModelState": {...} }` per research §6. Wired via `interpreter.NewFaultyResponder(errorFormats, statusCodeMapping)` where `statusCodeMapping` covers 401/403/404/422/429 → library's typed errors.

---

## Proxy

`Support.Proxy: true` on the ProviderInfo. No additional code in the deep package — the downstream gateway's generic passthrough consumes the authenticated HTTP client directly.

---

## Metadata

Static `providers/cyclrpartner/schemas.json` embedded via `//go:embed`, exposed through `schema.NewOpenAPISchemaProvider`. Covers `accounts`, `templates`, `connectors`.
