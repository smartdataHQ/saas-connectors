# Data Model: Cyclr Connector

Entities and attributes exposed by the two providers. **Field names preserve Cyclr's API casing (PascalCase) exactly, per CLAUDE.md §Naming and FR-050.** Where Cyclr's API wraps a response (e.g., `{"Accounts": [...]}`), the connector unwraps it for `ReadResult.Fields` but keeps the original in `ReadResult.Raw` (FR-051).

---

## Entity: Account (`accounts`)

**Scope**: `cyclrPartner`
**Endpoint base**: `/v1.0/accounts`
**Identifier**: `Id` (UUID, Cyclr-assigned, immutable)

### Attributes (write-visible unless noted)

| Field | Type | Mutable | Notes |
|---|---|---|---|
| `Id` | UUID string | no | Cyclr-assigned on create. |
| `Name` | string | yes | Required on create. |
| `Description` | string | yes | Optional. |
| `Timezone` | IANA TZ string | yes | Required on create (e.g., `Europe/London`). |
| `StepDataSuccessRetentionHours` | int | yes | Optional; data retention for successful step runs. |
| `StepDataErroredRetentionHours` | int | yes | Optional; data retention for errored step runs. |
| `TransactionErrorWebhookEnabled` | bool | yes | Optional. |
| `TransactionErrorWebhookUrl` | string | yes | Optional; required if `TransactionErrorWebhookEnabled` is true. |
| `TransactionErrorWebhookIncludeWarnings` | bool | yes | Optional. |
| `Enabled` / `IsSuspended` | bool | via suspend/resume actions, not update | Read-only on standard update path. |
| `CreatedOnUtc` | RFC3339 string | no | Server-assigned. |

### State transitions

```text
        create                         delete
  (none) ─────► Enabled ──────────────────────► (none)
                 │  ▲
         suspend │  │ resume
                 ▼  │
              Suspended ─────────────────────► (none)
                                         delete
```

- `create` produces `Enabled=true` by default.
- `suspend` toggles `Enabled=false`; Cycles in the Account stop running but configuration is retained.
- `resume` toggles `Enabled=true`; Cycles return to their prior activation state.
- `delete` is allowed from either state; cascading behaviour to Cycles is confirmed at Layer-2 (research §12.4).

### Validation (beyond what Cyclr itself enforces)

- `Name` non-empty (client-side short-circuit).
- `Timezone` must look like an IANA TZ (`*/*` or `UTC`); exact validation is delegated to Cyclr.
- If `TransactionErrorWebhookEnabled=true`, `TransactionErrorWebhookUrl` must be a valid absolute URL.

---

## Entity: Template (`templates`)

**Scope**: `cyclrPartner` (primary) and `cyclrAccount` (read-only view)
**Endpoint base**: `/v1.0/templates`
**Identifier**: `Id` (UUID)

### Attributes (all read-only from the connector's perspective)

| Field | Type | Notes |
|---|---|---|
| `Id` | UUID string | |
| `Name` | string | |
| `Description` | string | |
| `Connectors` | array of `{Id, Name, AuthenticationRequired}` | Third-party Connectors this template depends on. |
| `Category` | string | Optional. |
| `Version` | int | Optional. |
| `CreatedOnUtc` | RFC3339 string | |

Template creation/editing happens in the Cyclr Console and is explicitly out of scope for this connector (spec Assumptions).

---

## Entity: Connector (`connectors`)

**Scope**: `cyclrPartner`
**Endpoint base**: `/v1.0/connectors`
**Identifier**: `Id` (UUID)

### Attributes (read-only)

| Field | Type | Notes |
|---|---|---|
| `Id` | UUID string | |
| `Name` | string | e.g., "Salesforce", "HubSpot CRM". |
| `AuthenticationType` | enum string | e.g., `OAuth2`, `ApiKey`, `Basic`. |
| `Version` | int | Optional. |
| `IsPublic` | bool | Optional. |

This is the catalog of **third-party Connectors available** for installation into Accounts — not to be confused with Ampersand's own `connectors.go` interface definitions.

---

## Entity: Cycle (`cycles`)

**Scope**: `cyclrAccount`
**Endpoint base**: `/v1.0/cycles`
**Identifier**: `Id` (UUID)

### Attributes

| Field | Type | Mutable | Notes |
|---|---|---|---|
| `Id` | UUID string | no | |
| `Name` | string | no (post-install) | Inherited from template at install time. |
| `Status` | enum string | via activate/deactivate | Values observed: `Active`, `Paused`. |
| `TemplateId` | UUID string | no | Reference back to the source template. |
| `Interval` | int (minutes) | via activate | From a closed set: 1, 5, 15, 30, 60, 120, 180, 240, 360, 480, 720, 1440, 10080. |
| `StartTime` | RFC3339 string | via activate | Optional. |
| `RunOnce` | bool | via activate | When true, Cycle auto-pauses after first run. |
| `Connectors` | array of `{Id, Name, AccountConnectorId}` | no | Connector installations this Cycle depends on. |
| `ErrorCount` | int | no | Validation-time errors. |
| `WarningCount` | int | no | Validation-time warnings. |
| `CreatedOnUtc` | RFC3339 string | no | |

### State transitions

```text
  (none) ───────► Paused ──────────────► Active
        install          activate
                  ◄──────────────
                    deactivate
            ─────────────────────► (none)
                   delete
```

- Install (from template) produces `Status=Paused` with `ErrorCount`/`WarningCount` populated.
- Activate requires validation errors to be zero (Cyclr-enforced); the connector surfaces Cyclr's refusal message (edge case).
- Delete is allowed from either state.

### Validation

- `Interval` must be one of the allowed closed set when activating.
- `StartTime`, if present, must parse as RFC3339 UTC.

---

## Entity: AccountConnector (`accountConnectors`)

**Scope**: `cyclrAccount`
**Endpoint base**: `/v1.0/account/connectors` (exact path verified at Layer-2)
**Identifier**: `Id` (UUID)

Represents a specific installation of a third-party `Connector` inside this Account.

### Attributes (read-only in MVP)

| Field | Type | Notes |
|---|---|---|
| `Id` | UUID string | The `AccountConnectorId` referenced in Cycle responses. |
| `ConnectorId` | UUID string | Reference to the catalog `Connector`. |
| `Name` | string | Often `{ConnectorName} ({account display})`. |
| `AuthenticationState` | enum string | `Authenticated`, `AwaitingAuthentication`, or similar. |
| `CreatedOnUtc` | RFC3339 string | |

**Explicit exclusion**: stored credential material for this installation is **never exposed** by the connector (FR-032, FR-062). If Cyclr's API returns credential values in this response, the connector strips them from `Fields` (but `Raw` preserves whatever Cyclr sends so callers can inspect shape — and if secrets are leaking at the API level that's a Cyclr bug to address upstream, not ours to compound).

---

## Relationships

```text
Partner (1) ──── (N) Account
                     │
           suspend/  │
           resume    │
                     ├── (1) ──── (N) Cycle ─────── (1) Template
                     │                    \_______ (N) AccountConnector
                     │
                     └── (1) ──── (N) AccountConnector ─── (1) Connector

Template ──── (N) ──── Connector       (template depends on Connectors)
```

- An `Account` has many `Cycles` and many `AccountConnector` installations.
- A `Cycle` references exactly one `Template` (source) and zero or more `AccountConnectors` (runtime dependencies).
- A `Template` lists the third-party `Connectors` it needs — installing a Template auto-installs missing `AccountConnectors` (per Cyclr docs).

## Pagination shape (all list endpoints)

Request: `?page=<N>&per_page=50` (1-indexed). Response body is an array or a wrapped object `{ "X": [...], "TotalPages": N, "TotalRecords": M }` (exact shape per endpoint — contracts file pins each one). Pagination state is carried in the response headers `Total-Pages` and `Total-Records` (name confirmation deferred to Layer-2, research §12.1).

`NextPageFunc` returns `""` when the current page ≥ `Total-Pages`, otherwise returns the next page number as a string.

## Secret handling recap (cross-cutting)

| Field | Logged? | In `Fields`? | In `Raw`? |
|---|---|---|---|
| OAuth `client_secret` | **Never** | Never | Never |
| OAuth `access_token` | **Never** | Never | Never |
| Account `Id` (API ID) | Yes (structured) | Yes | Yes |
| `AccountConnector` stored credentials | Never | Never | If Cyclr returns them, stripped from `Fields`; `Raw` passes through — flagged to Cyclr if observed |
| Webhook URL | Yes | Yes | Yes |
