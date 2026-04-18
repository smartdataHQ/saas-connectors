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
**Endpoint base**: `/v1.0/connectors` for the install action (`POST /v1.0/connectors/{id}/install`); `/v1.0/account/connectors` (exact path confirmed at Layer-2) for list/read.
**Identifier**: `Id` (UUID)

Represents a specific installation of a third-party `Connector` inside this Account.

### Attributes (read)

| Field | Type | Notes |
|---|---|---|
| `Id` | UUID string | The `AccountConnectorId` referenced in Cycle responses. |
| `ConnectorId` | UUID string | Reference to the catalog `Connector`. |
| `Name` | string | Often `{ConnectorName} ({account display})`. |
| `AuthenticationState` | enum string | `Authenticated`, `AwaitingAuthentication`, or similar. |
| `CreatedOnUtc` | RFC3339 string | |

### Attributes (create — install into Account)

The create body accepts these fields (shape taken from Cyclr's `POST /v1.0/connectors/{id}/install`):

| Field | Type | Mandatory | Notes |
|---|---|---|---|
| `Name` | string | yes | Display name for this installation. |
| `Description` | string | no | |
| `AuthValue` | string | conditional | API Keys: plain text. Basic Auth: base64 `user:pass`. OAuth: omit — creation succeeds in "awaiting authorisation" state and the browser-redirect flow is handled outside this connector. |

The catalog `Connector`'s authentication type (from the entity above) determines what `AuthValue` should contain. `AuthValue` is never logged or echoed back in responses (FR-034).

### State transitions

```text
       install                                  (no delete in typed MVP)
 (none) ──────► AwaitingAuthentication ──► Authenticated
                    │                           │
                    │  (browser OAuth flow,     │
                    │   outside this connector) │
                    └───────────────────────────┘
```

- API-key / Basic Connectors are typically `Authenticated` immediately after install.
- OAuth Connectors are `AwaitingAuthentication` after install until the sign-in-token redirect completes in the caller's UI.

### Validation

- `Name` non-empty.
- For API-key / Basic: `AuthValue` present and non-empty.
- For OAuth: `AuthValue` may be omitted; if present, it is ignored (Cyclr rejects it in this flow).

### Secret handling

**Explicit exclusion on read**: stored credential material for this installation is **never exposed** by the connector (FR-032, FR-034, FR-062). If Cyclr's API returns credential values in a read response, the connector strips them from `Fields`. `Raw` preserves whatever Cyclr sends (so callers can inspect shape).

**Explicit exclusion on create**: the caller-supplied `AuthValue` is passed through to Cyclr but is never written to logs, error messages, or telemetry.

---

## Entity: CycleStep (`cycleSteps`)

**Scope**: `cyclrAccount`
**Endpoint base**:
- List (per Cycle): `GET /v1.0/cycles/{cycleId}/steps` — exposed via parent-scoped object name pattern `cycles/{cycleId}/steps` OR via pass-through until a cleaner ergonomic is chosen at Layer-2 (see contract).
- By id: `GET /v1.0/steps/{stepId}` — exposed as typed `cycleSteps` read with `RecordId = stepId`.
**Identifier**: `Id` (UUID)

Represents a single node within an installed Cycle — an action, trigger, or control step. **Read-only via the typed surface in MVP** (FR-026). Writes to Step parameters and field mappings remain pass-through.

### Attributes (read)

| Field | Type | Notes |
|---|---|---|
| `Id` | UUID string | Step identifier. |
| `Name` | string | Human-readable step name inherited from the template. |
| `CycleId` | UUID string | The parent Cycle. |
| `ConnectorId` | UUID string | The third-party Connector this Step targets. |
| `AccountConnectorId` | UUID string | The specific `AccountConnector` installation this Step uses. |
| `MethodName` | string | The Connector method invoked by this Step (e.g., `CreateContact`). |
| `StepType` | enum string | e.g., `Action`, `Trigger`, `Control`. |
| `ErrorCount` | int | Validation-time errors. |
| `WarningCount` | int | Validation-time warnings. |

Nested under `Method` (when returned by Cyclr):

- `Parameters` — array of Step-parameter descriptors (`Id`, `Name`, mapping type, current value). Read-only here; writes are pass-through.
- `RequestFields` — array of field-mapping descriptors (same shape, different attachment point — request body vs URL/header).

### Secret handling

Step responses may include the current values of mapped parameters/fields. If any value resembles credential material (matches names like `AccessToken`, `ApiKey`, `Password`, `Bearer`, or carries a schema hint indicating secret), it is stripped from `Fields`. `Raw` is preserved (FR-028).

---

## Entity: StepParameter (`stepParameters`)

**Scope**: `cyclrAccount`
**Endpoint base**:
- List (per Step): `GET /v1.0/steps/{stepId}/parameters` — exposed as parent-scoped object name `steps/{stepId}/parameters`.
- By id: `GET /v1.0/steps/{stepId}/parameters/{parameterId}` — exposed as `stepParameters` with `RecordId = parameterId` and `StepId` provided via `RecordData.StepId` (read) or extracted from `params.AssociatedID`/equivalent (verified at Layer-2; may require compound `RecordId = "{stepId}:{parameterId}"` if ReadParams lacks a parent-id field).
- Update: `PUT /v1.0/steps/{stepId}/parameters/{parameterId}` — same addressing.

**Identifier**: `Id` (UUID), always scoped to a Step.

### Attributes

| Field | Type | Mutable | Notes |
|---|---|---|---|
| `Id` | UUID string | no | Parameter identifier, unique per Step. |
| `StepId` | UUID string | no | Parent Step (included in read responses for caller convenience). |
| `Name` | string | no | Parameter display name from the Connector method. |
| `MappingType` | enum string | yes | One of: `StaticValue`, `ValueList`, `StepOutput`, `AccountVariable`, plus any additional types Cyclr introduces (passed through uninterpreted per FR-037). |
| `Value` | string | yes | For `StaticValue` and `ValueList`: the literal or selected value. For `StepOutput`: unused (see `SourceStepId` + `SourceFieldName`). For `AccountVariable`: unused (see `VariableName`). |
| `SourceStepId` | UUID string | yes | Used with `MappingType: StepOutput` — the upstream Step whose output is piped in. |
| `SourceFieldName` | string | yes | Used with `MappingType: StepOutput` — the field name on the upstream Step's output. |
| `VariableName` | string | yes | Used with `MappingType: AccountVariable` — name of the Account Variable. |
| `AllowedValues` | array of string | no | Populated for `MappingType: ValueList` parameters — enumerates valid choices. |

### Validation

- `MappingType` must be non-empty on update.
- For `MappingType: ValueList` updates, `Value` should be in `AllowedValues` (client-side short-circuit if the list is known; otherwise Cyclr rejects with 422 and the connector surfaces the error).
- For `StepOutput`, both `SourceStepId` and `SourceFieldName` must be present.
- For `AccountVariable`, `VariableName` must be present.

### Secret handling

If a parameter's current `Value` resembles credential material (name-based heuristic: `AccessToken`, `ApiKey`, `Password`, `Bearer`, `Secret`, `Token`), it is stripped from `Fields`. `Raw` preserves. On update, the submitted value is never logged or echoed (FR-039).

---

## Entity: StepFieldMapping (`stepFieldMappings`)

**Scope**: `cyclrAccount`
**Endpoint base**:
- List (per Step): `GET /v1.0/steps/{stepId}/fieldmappings` — exposed as parent-scoped object name `steps/{stepId}/fieldmappings`.
- By id: `GET /v1.0/steps/{stepId}/fieldmappings/{fieldId}` — analogous addressing to StepParameter.
- Update: `PUT /v1.0/steps/{stepId}/fieldmappings/{fieldId}`.

**Identifier**: `Id` (UUID), scoped to a Step.

### Attributes

Same as StepParameter. Difference is purely in how Cyclr attaches the resolved value to the outbound third-party request (body vs header/URL). From an agent's perspective the shape and mapping types are identical; the split exists because Cyclr models them on separate endpoints.

### Relationship to StepParameter

Both are **Step inputs**, differing only by attachment point. An MCP tool generator might choose to collapse them into a single "Step Input" tool group with an `inputKind: parameter | fieldmapping` discriminator; the connector surfaces them as distinct objects because Cyclr's API does.

---

## Entity: StepPrerequisites (synthetic `cycleSteps:prerequisites`)

**Scope**: `cyclrAccount`
**Endpoint**: `GET /v1.0/steps/{stepId}/prerequisites`
**Identifier**: none — always scoped to a Step identifier supplied via `RecordId`.

Diagnostic view (FR-027) that identifies which Step parameters, field mappings, or authentications are missing or awaiting configuration. Intended to let an operator diagnose why `cycles:activate` would fail before they invoke it.

### Attributes (read)

Shape is Cyclr-defined; the connector preserves whatever Cyclr returns (typically an array of missing-prerequisite descriptors with `Type`, `Name`, `Reason` fields). Exact field names confirmed at Layer-2.

### Secret handling

Same as CycleStep — no credential values surface in `Fields`.

---

## Relationships

```text
Partner (1) ──── (N) Account
                     │
           suspend/  │
           resume    │
                     ├── (1) ──── (N) Cycle ─────── (1) Template
                     │                 │     \______ (N) AccountConnector  (runtime deps)
                     │                 │
                     │                 └── (N) CycleStep ───────── (1) AccountConnector
                     │                              │
                     │                              └── (diagnostic) StepPrerequisites
                     │
                     └── (1) ──── (N) AccountConnector ─── (1) Connector

Template ──── (N) ──── Connector       (template declares required Connectors)
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
| `AccountConnector` stored credentials (on read) | Never | Never | If Cyclr returns them, stripped from `Fields`; `Raw` passes through — flagged to Cyclr if observed |
| `AccountConnector.AuthValue` (on create) | Never | N/A (write path) | Not echoed back in response |
| `CycleStep` parameter/field-mapping values resembling credentials | Never | Stripped | Preserved in `Raw` |
| Webhook URL | Yes | Yes | Yes |
