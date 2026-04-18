# Feature Specification: Cyclr Connector

**Feature Branch**: `149-cyclr-connector`
**Created**: 2026-04-18
**Status**: Draft
**Input**: User description: "Cyclr Connector"

## Context

Cyclr is a workflow / embedded-iPaaS platform. In Cyclr terminology, "Cycles" are workflows and "Accounts" are tenants under a Partner. As the Partner (operator/owner) we create and operate white-label Accounts on behalf of our customers and install/manage Cycles inside those Accounts.

The `saas-connectors` library (this repo) powers the downstream `fraios/apps/saas-gateway`, which is consumed by `cxs2`-based MCP services. Any capability delivered here becomes callable by:

1. The **gateway's own API user** (privileged operator) — must be able to perform every supported Cyclr operation.
2. **Organisation / owner** (Partner role inside Cyclr) — must be able to create, maintain, and operate customer Accounts and the Cycles inside them.

This feature adds first-class Cyclr support to the `saas-connectors` library so those roles can call Cyclr through the gateway instead of hitting Cyclr's API directly.

## Clarifications

### Session 2026-04-18

- Q: Should the Cyclr connector ship as one catalog entry covering both scopes, or as two separate catalog entries (one per scope)? → A: Two providers — `cyclrPartner` (Account lifecycle, catalog) and `cyclrAccount` (Cycle lifecycle, Account-scoped reads). Each is single-scope. Rationale: Partner-level and Account-level Cyclr calls use different OAuth token scopes and different request headers; treating them as separate providers matches the library's "one connection = one auth context" convention and avoids per-call scope dispatch.
- Q: Is the Cyclr Account API-ID a secret or an identifier for logging/telemetry purposes? → A: Identifier. May appear in structured logs, error messages, span attributes, and audit events. Only the bearer token and client secret are treated as credentials. Rationale: the API-ID grants nothing on its own (a valid bearer token is still required), and observability of which Account a request targeted is operationally important.
- Q: Should Account suspend/resume (disable without deleting) be in MVP? → A: Yes, include. Suspend and resume are added alongside create/update/delete on the `cyclrPartner` provider. Rationale: "maintain" in the user's ask naturally includes pausing a delinquent or offboarding customer without destroying data, and excluding it would force operators back to the Cyclr Console for a common lifecycle step.
- Q: What is the concrete retry budget for rate-limit (429) responses from Cyclr? → A: Up to 3 total attempts per caller-visible request. Honor Cyclr's `Retry-After` header when present; otherwise exponential backoff with jitter starting at ~1s (e.g., 1s / 2s / 4s). Wall-clock ceiling ~30 seconds per caller-visible request — if the budget is exhausted, surface the 429 to the caller. Rationale: keeps the gateway's per-request timeout predictable and balances burst tolerance against hiding prolonged outages.
- Q: Is Cyclr's Data-on-Demand API (invoking an installed Connector method without a Cycle) in scope for MVP, and if so how? → A: Pass-through only. MVP does not expose a typed read/write surface for Data-on-Demand; callers invoke it via `cyclrAccount`'s pass-through surface. Rationale: Data-on-Demand is RPC-over-catalog (per-Connector-method argument schemas), a different paradigm from CRUD on stable resources. Typing it properly requires dynamic per-method schema discovery and would delay the core Account + Cycle lifecycle value. Re-evaluate for typed coverage after MVP.
- Q (scope-expansion, post-gap-analysis): After peer review against Cyclr's full API surface, should MVP include typed read for Cycle Steps + step Prerequisites, and typed write for installing an AccountConnector? → A: Yes. Add FR-026..028 (Cycle Steps read, step Prerequisites read) and FR-033 (AccountConnector install). Rationale: without Cycle Steps introspection, "operate Cycles" degenerates to install-and-run-unchanged; without Prerequisites, FR-022 ("activate a Cycle") can only fail on the Cyclr side with no way for the caller to diagnose ahead of time; without AccountConnector install, a template whose required Connectors are not yet installed cannot be installed at all without dropping to pass-through. Step-parameter and step-field-mapping *writes* remain pass-through only (configuration-as-code is a much larger surface and can be typed later if demand materialises).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - White-label Account lifecycle (Priority: P1)

As the Partner operator, I onboard a new customer by creating a white-label Cyclr Account for them, configuring its retention and error-notification settings, maintaining it over time (update description, timezone, webhook URLs), and eventually suspending or deleting it when the customer churns. I do this without opening the Cyclr Console — my tooling calls the connector.

**Why this priority**: This is the precondition for everything else. Without an Account, there are no Cycles to operate. It also captures the highest-value white-label moment — onboarding — where manual steps most hurt time-to-value.

**Independent Test**: Operator creates an Account via the connector, confirms the new Account is visible in the Cyclr Partner Console, updates one or more of its fields via the connector, confirms the update, and deletes it. No Console interaction required.

**Acceptance Scenarios**:

1. **Given** valid Partner-level credentials, **When** the operator asks the connector to create an Account with a name, timezone, and retention settings, **Then** a new Account exists in Cyclr, the connector returns the Account's identifier, and the Account's attributes match what was requested.
2. **Given** an existing Account, **When** the operator asks the connector to list Accounts, **Then** the response includes that Account with its current attributes and supports paging through additional Accounts if the Partner has many.
3. **Given** an existing Account, **When** the operator asks the connector to update the Account's description, webhook URL, or retention, **Then** the update is persisted and the next read returns the new values.
4. **Given** an existing Account that needs to be paused (e.g., billing delinquency), **When** the operator asks the connector to suspend it, **Then** Cyclr marks the Account as disabled, its Cycles stop running, and a subsequent read reflects the disabled state; the Account's data and configuration remain intact.
5. **Given** a suspended Account whose customer has resumed, **When** the operator asks the connector to resume it, **Then** Cyclr re-enables the Account and its previously active Cycles resume operating on their triggers.
6. **Given** an existing Account no longer needed, **When** the operator asks the connector to delete it, **Then** Cyclr removes the Account and subsequent reads no longer return it.

---

### User Story 2 - Operate Cycles inside a customer Account (Priority: P1)

As the Partner operator acting on behalf of a specific customer Account, I install a Cycle (workflow) from a catalog template into that Account, configure/authorise the steps the Cycle needs, activate the Cycle so it runs on its trigger, deactivate it for maintenance, and remove it when no longer needed. I can list the Cycles currently installed in an Account and see their activation state.

**Why this priority**: Operating Cycles is the core product value we deliver to customers. Account creation without Cycle operations is an empty shell. P1 alongside Story 1 because the two together form the minimum viable outcome.

**Independent Test**: With a test Account already present, the operator installs a Cycle from a known template, activates it, lists Cycles, confirms the installed Cycle appears and is marked active, deactivates it, confirms the state change, then removes it. No Console interaction required.

**Acceptance Scenarios**:

1. **Given** an existing Account and a known Cycle template, **When** the operator installs the template into that Account via the connector, **Then** a new Cycle exists in the Account and the connector returns its identifier.
2. **Given** an installed but inactive Cycle, **When** the operator asks the connector to activate it, **Then** Cyclr reports the Cycle as active on subsequent reads and it begins responding to its trigger.
3. **Given** an active Cycle, **When** the operator asks the connector to deactivate it, **Then** Cyclr reports the Cycle as inactive and it stops responding to its trigger.
4. **Given** an existing Account, **When** the operator asks the connector to list its Cycles, **Then** the response includes every installed Cycle with at minimum its identifier, name, and activation state, and supports paging.
5. **Given** an installed Cycle that is no longer needed, **When** the operator asks the connector to delete it, **Then** Cyclr removes the Cycle from the Account and subsequent reads no longer return it.
6. **Given** an installed Cycle whose activation has failed or is uncertain, **When** the operator asks the connector to list that Cycle's Steps and, for any Step, read its Prerequisites, **Then** the response identifies which Step parameters or field mappings are missing, or which Connector authentications are awaiting, so the operator can see *why* the Cycle is not ready.
7. **Given** a template whose required third-party Connectors are not yet installed in the target Account, **When** the operator first installs the missing Connectors into the Account via the connector (providing a name and, where applicable, an authentication value), **Then** each Connector becomes present as an AccountConnector in the Account, and the subsequent template install succeeds.
8. **Given** an installed Cycle whose Step parameters or field mappings need to be reconfigured (e.g., to point at a different AccountConnector, to change a static value, to re-wire a data flow from a prior Step's output), **When** an operator or MCP agent calls the connector to update the relevant Step parameter or field mapping with a new `MappingType` and value, **Then** the change is persisted on Cyclr and a subsequent read of that parameter/mapping reflects the new value without requiring any Cyclr Console interaction.

---

### User Story 3 - Catalog and connector visibility (Priority: P2)

As the Partner operator, I discover which Cycle templates and which third-party Connectors are available to install into an Account, so I can make an informed choice before installing. I may also list the Connector installations currently present in a given Account.

**Why this priority**: Strictly speaking, an operator who already knows the template ID can skip this. But without discoverability the operator has to cross-reference the Cyclr Console to find IDs, which breaks the "never open the Console" promise of Stories 1 and 2. P2 because Stories 1+2 can ship first and this fills the gap.

**Independent Test**: Operator lists available templates, lists available Connectors, picks one of each, and confirms the returned identifiers work as inputs to the Story 2 install flow.

**Acceptance Scenarios**:

1. **Given** valid Partner-level credentials, **When** the operator asks the connector to list Cycle templates, **Then** the response includes each template's identifier and a human-readable name, paged as needed.
2. **Given** valid Partner-level credentials, **When** the operator asks the connector to list available third-party Connectors, **Then** the response includes each Connector's identifier and name.
3. **Given** an existing Account, **When** the operator asks the connector to list the Connector installations in that Account, **Then** the response identifies each installed Connector and its authentication state (authorised or awaiting authorisation), without exposing stored credentials.

---

### User Story 4 - Passthrough for advanced operations (Priority: P2)

As the Partner operator, when I need to call a Cyclr endpoint that is not yet typed in the connector's read/write/delete surface (for example, configuring an individual Cycle step, setting an Account variable, or calling a Data-on-Demand endpoint), I can use the connector in a pass-through mode to make the raw HTTP call with authentication handled for me.

**Why this priority**: The Cyclr API surface is large and evolves. Without pass-through, any gap in the typed surface becomes a blocker and forces out-of-band integration. With pass-through, the gateway user can exercise the full Cyclr API on day one while we incrementally add typed coverage for the common operations.

**Independent Test**: Operator sends a raw authenticated request to a Cyclr endpoint that is not in the typed surface (for example, a step configuration update) via the connector's pass-through mode and receives the Cyclr response verbatim.

**Acceptance Scenarios**:

1. **Given** valid credentials stored in the gateway, **When** the gateway API user issues a pass-through request to any Cyclr path, **Then** the connector attaches the correct authentication headers and returns Cyclr's response unchanged (status, headers relevant to the caller, and body).
2. **Given** a pass-through request against an Account-scoped endpoint, **When** the credential is configured for a specific Account, **Then** the connector attaches the Account context automatically so the operator does not need to inject it per call.

---

### Edge Cases

- **Expired or invalid credentials.** Cyclr tokens are short-lived on their own but the Partner grant is stable; when a stored token has expired, the connector must refresh transparently without failing the caller's request, and must surface a clear, actionable error if the underlying client credentials are themselves revoked.
- **Partner vs Account confusion.** A request aimed at an Account-scoped endpoint but issued with a Partner-only credential (or vice versa) must fail with a clear error that names the required scope, not a raw HTTP 401/403 from Cyclr.
- **Region mismatch.** Cyclr runs multiple regional API instances (e.g., `api.cyclr.com`, `api.eu.cyclr.com`, `api.us2.cyclr.com`, private). A credential created against one region must never be sent to another; misconfiguration must be detected early, not leak into request logs.
- **Pagination past the end.** Listing Accounts, Cycles, or templates when there are zero results or when the caller pages past the final page must return an empty result and a clear "no more pages" signal, not an error.
- **Template install of an unknown or inaccessible template.** The connector must surface Cyclr's specific error (template not visible to this Partner, template mis-versioned, Account lacks required Connectors) rather than a generic failure.
- **Activating a Cycle that is incompletely configured.** Cyclr rejects activation when required step configuration or authorisations are missing. The connector must return the Cyclr-reported reason so the caller can act on it.
- **Deleting an Account that still has active Cycles.** If Cyclr rejects the delete, the connector must surface that precondition failure; if Cyclr allows cascading delete, the behaviour must be documented so the operator is not surprised.
- **Rate limits.** Cyclr enforces rate limits; bursts of white-label onboarding (create Account + install several Cycles) must not trip them in normal usage and must back off gracefully when they do.
- **Concurrent updates.** Two operators updating the same Account or Cycle near-simultaneously must not silently clobber each other; last-writer-wins is acceptable only if the response makes it visible.
- **Pass-through requests to privileged paths.** The pass-through surface must not allow escalation beyond the credential's scope — an Account-scoped credential cannot be used to reach Partner-only endpoints even via pass-through.

## Requirements *(mandatory)*

### Functional Requirements

> **Numbering convention**: FRs are grouped in blocks (010s = Account lifecycle, 020s = Cycle lifecycle + Steps, 030s = Catalog + AccountConnector install, 035+ = Step configuration, 045+ = MCP metadata + taxonomy, 040s = Pass-through, 050s = Library conventions, 060s = Errors). Gaps in numbering (e.g., FR-006..009, FR-019, FR-029, FR-043..044, FR-054..059) are intentional — each block leaves space for future additions without re-numbering. Absence of a specific FR number therefore does not indicate a missing requirement.

#### Authentication & connection

- **FR-001**: The connector MUST authenticate to Cyclr using OAuth 2.0 Client Credentials. No other auth mode is in scope.
- **FR-002**: The connector MUST support Cyclr's multiple regional API instances (including private instances) by taking the API domain as a connection-time input, and MUST refuse to call any host outside that configured domain.
- **FR-003**: The feature MUST ship as two independent provider entries, each single-scope:
  - `cyclrPartner` — Partner-level. No specific Account context. Carries only Partner-scoped credentials. Exposes Account lifecycle (FR-010..015), catalog (FR-030..031), and Partner-scoped pass-through (FR-040..041).
  - `cyclrAccount` — Account-level. Scoped at connection creation to exactly one Account identifier, with the Account context header applied to every call. Carries Account-scoped credentials. Exposes Cycle lifecycle (FR-020..025), installed-Connector visibility (FR-032), and Account-scoped pass-through (FR-040..042).
  A single connection MUST NOT switch scope at runtime. To operate across many customer Accounts, the gateway creates one `cyclrAccount` connection per Account.
- **FR-004**: The connector MUST transparently refresh or re-issue access tokens before they expire so that consumers never observe an expiry-driven failure under normal operation.
- **FR-005**: The connector MUST return a clear, typed error (not an opaque HTTP status) when a request cannot be served because the connection's scope is wrong for the requested operation.

#### Account lifecycle (Partner scope)

- **FR-010**: The connector MUST allow listing Accounts belonging to the Partner, with paging.
- **FR-011**: The connector MUST allow creating an Account with at minimum: name, description (optional), timezone, step-data retention for successes, step-data retention for errors, transaction-error webhook enable/URL/include-warnings.
- **FR-012**: The connector MUST return the new Account's unique identifier and creation timestamp on successful create.
- **FR-013**: The connector MUST allow updating any mutable Account attribute from FR-011 on an existing Account, addressed by its identifier.
- **FR-014**: The connector MUST allow deleting an Account by its identifier and MUST surface Cyclr's response (including refusal when preconditions aren't met) without masking.
- **FR-015**: The connector MUST allow reading a single Account by its identifier and return the same attributes as the list form.
- **FR-016**: The connector MUST allow suspending (disabling) an existing Account by its identifier without deleting its data. A suspended Account's installed Cycles MUST stop responding to their triggers.
- **FR-017**: The connector MUST allow resuming (re-enabling) a previously suspended Account by its identifier, restoring its Cycles to the activation state they held prior to suspension.
- **FR-018**: Account reads (list and single) MUST expose the Account's current enabled/suspended state so operators can reason about why Cycles in an Account may not be running.

#### Cycle lifecycle (Account scope)

- **FR-020**: The connector MUST allow listing the Cycles installed in the Account the credential is scoped to, with paging, including each Cycle's identifier, name, and activation state.
- **FR-021**: The connector MUST allow installing a Cycle into the Account from an existing Cycle template identifier, and MUST return the newly-created Cycle's identifier.
- **FR-022**: The connector MUST allow activating an installed Cycle by its identifier.
- **FR-023**: The connector MUST allow deactivating an installed Cycle by its identifier.
- **FR-024**: The connector MUST allow deleting an installed Cycle by its identifier.
- **FR-025**: The connector MUST allow reading a single Cycle by its identifier with at minimum the attributes returned in FR-020.
- **FR-026**: The connector MUST allow listing the Steps of an installed Cycle (identified by the Cycle's identifier) and reading a single Step by its own identifier. Step reads MUST include at minimum the Step's identifier, name, the Connector/method it calls, and any validation counts (error/warning) if Cyclr exposes them.
- **FR-027**: The connector MUST allow reading the Prerequisites of a given Step by the Step's identifier. The response MUST identify, at minimum, which Step parameters, field mappings, or authentications are missing or awaiting configuration, so an operator can resolve activation-time failures without opening the Cyclr Console.
- **FR-028**: Reads of Cycle Steps and Step Prerequisites MUST NOT expose any credential material stored against the Step or its dependent Connector installations (consistent with FR-032, FR-062).

#### Step configuration (for MCP agents and operators)

Cycles installed from templates are runnable only when every Step has its parameters and field mappings correctly populated. These requirements let an MCP agent or operator reconfigure an installed Cycle without opening the Cyclr Console.

- **FR-035**: The connector MUST allow listing the parameters of a given Step (identified by the Step's identifier), returning for each parameter its identifier, name, current mapping type, and current mapped value (or empty when unmapped).
- **FR-036**: The connector MUST allow reading a single Step parameter addressed by the pair (Step identifier, parameter identifier), with the same fields as the list form.
- **FR-037**: The connector MUST allow updating a single Step parameter's mapping. The update MUST accept at minimum the following `MappingType` variants: `StaticValue` (with a literal `Value`), `ValueList` (with a `Value` drawn from the parameter's allowed list), `StepOutput` (with `SourceStepId` and `SourceFieldName` referencing an upstream Step's output), and `AccountVariable` (with `VariableName`). Additional mapping types Cyclr supports MUST be passed through uninterpreted rather than rejected.
- **FR-038**: The connector MUST allow listing, reading, and updating a Step's **field mappings** (the subset of Step inputs attached to the request body, as opposed to parameters attached to headers or URL). The shape and `MappingType` variants MUST match FR-035..037. Field mappings and parameters are distinct Cyclr endpoints but share the same conceptual shape.
- **FR-039**: Step parameter and field-mapping reads MUST apply the same credential-stripping heuristic as Step reads (FR-028) — values resembling credentials are removed from `Fields` while preserved in `Raw`. Update operations MUST NOT echo the submitted value back in any log, error, or telemetry line.

#### Metadata surface for downstream tool generation (MCP-facing)

The gateway that consumes this library (`fraios/apps/saas-gateway`) generates MCP tool schemas from the connector's `ObjectMetadata` + `FieldMetadata`. Tool ergonomics for agents therefore depend directly on how rich this connector's metadata is. FR-045..048 codify the minimum metadata discipline for this feature.

- **FR-045**: Every object exposed by either provider MUST have a populated `DisplayName` in its `ObjectMetadata`. The DisplayName MUST be short, human-readable, and suitable as an MCP tool title fragment (e.g., `"Cyclr Account"`, `"Cycle Step"`, `"Step Parameter"`).
- **FR-046**: Every field exposed in `FieldMetadata` MUST have a populated `DisplayName`, a `ValueType` (not empty), and a `ProviderType` matching Cyclr's actual type label for that field. `IsRequired` MUST be set (true/false) for every field used as input on a create or update path. `ReadOnly` MUST be set for fields that cannot be written.
- **FR-047**: Closed-set fields MUST populate the `Values` enum. At minimum: `Account.Timezone` (IANA list is open-ended and exempt), `Cycle.Status` (`Active`, `Paused`), `Cycle.Interval` (the closed minute set from FR-022's Cyclr contract), `CycleStep.StepType` (`Action`, `Trigger`, `Control`), `AccountConnector.AuthenticationState` (`Authenticated`, `AwaitingAuthentication`), and every `MappingType` field in Step parameters / field mappings.
- **FR-048**: Lookup / reference fields MUST populate `ReferenceTo` with the object names they reference (e.g., `Cycle.TemplateId.ReferenceTo = ["templates"]`, `CycleStep.CycleId.ReferenceTo = ["cycles"]`, `CycleStep.AccountConnectorId.ReferenceTo = ["accountConnectors"]`). This lets MCP tool generators link related tools together.

#### Object-name taxonomy (MCP tool-grouping convention)

- **FR-049**: Object names MUST follow a stable taxonomy so downstream MCP tool grouping by prefix / suffix is deterministic:
  - **Account lifecycle**: `accounts`, `accounts:suspend`, `accounts:resume` on `cyclrPartner`.
  - **Cycle control**: `cycles`, `cycles:activate`, `cycles:deactivate` on `cyclrAccount`.
  - **Cycle diagnostics**: `cycleSteps`, `cycleSteps:prerequisites`, parent-scoped list `cycles/{cycleId}/steps` on `cyclrAccount`.
  - **Step configuration**: `stepParameters`, `stepFieldMappings`, parent-scoped lists `steps/{stepId}/parameters` and `steps/{stepId}/fieldmappings` on `cyclrAccount`.
  - **Connector setup**: `accountConnectors` on `cyclrAccount`.
  - **Catalog**: `templates`, `connectors` on `cyclrPartner`; `templates` (read-only view) on `cyclrAccount`.

  This taxonomy MUST be stable across minor releases — downstream MCP tool IDs depend on it. Renaming a path is a breaking change for the gateway.

#### Catalog & visibility

- **FR-030**: The connector MUST allow listing Cycle templates visible to the Partner, with paging, returning each template's identifier and name.
- **FR-031**: The connector MUST allow listing third-party Connectors visible to the Partner or Account (depending on scope), returning each Connector's identifier and name.
- **FR-032**: The connector MUST allow listing Connector installations present in the scoped Account, returning each installation's identifier, Connector reference, and authorisation state, without exposing stored secrets.
- **FR-033**: The connector MUST allow installing a third-party Connector from the catalog into the scoped Account, creating a new AccountConnector. The create call MUST accept at minimum a display name, an optional description, and (for API-key or Basic-auth Connectors) an authentication value — the same fields Cyclr's API accepts. For OAuth-backed Connectors, the connector MUST NOT attempt to complete the OAuth dance itself; the typed write MUST succeed on the API-side creation and return an AccountConnector whose authorisation state is "awaiting authorisation", leaving the browser-based OAuth completion to a separate caller-side flow (documented in pass-through / the gateway's responsibility).
- **FR-034**: The connector MUST NOT accept or forward stored credential material (API keys, basic-auth passwords, OAuth tokens) in any response or log line emitted from AccountConnector reads; only the authorisation state and non-sensitive metadata are surfaced. On create, the caller-supplied authentication value is used only to build the outbound Cyclr request and MUST NOT be echoed back into logs, errors, or telemetry.

#### Pass-through

- **FR-040**: The connector MUST expose a pass-through mode that forwards arbitrary HTTP requests (any verb, any path under the configured Cyclr API domain) to Cyclr with the connection's authentication applied and returns Cyclr's response body verbatim.
- **FR-041**: Pass-through MUST reject requests targeting any host other than the configured Cyclr API domain.
- **FR-042**: When the connection is Account-scoped, pass-through MUST inject the Account context header so callers do not need to include it themselves, and MUST NOT allow it to be overridden to a different Account.

#### Consistency with library conventions

- **FR-050**: The connector MUST preserve Cyclr's field names and object names exactly as they appear in Cyclr's API (no renaming, no case coercion).
- **FR-051**: For every read operation, the connector MUST return both a flattened/processed record and the unmodified original response, consistent with how every other typed connector in this library behaves.
- **FR-052**: For every write operation, the connector MUST accept payloads in the same shape as the corresponding read form, performing any Cyclr-specific re-wrapping internally.
- **FR-053**: The connector MUST declare only the capabilities (read / write / delete / proxy / metadata) it actually implements; it MUST NOT advertise capabilities that are not wired.

#### Errors & resilience

- **FR-060**: The connector MUST translate Cyclr's error shapes (including 4xx, 5xx, and any 200-with-error-body cases) into the library's typed error categories so consumers can distinguish "bad input", "auth problem", "not found", "rate limit", and "upstream failure".
- **FR-061**: On rate-limit (HTTP 429) responses from Cyclr, the connector MUST retry transparently up to **3 total attempts** per caller-visible request. It MUST honor Cyclr's `Retry-After` header when present; otherwise it MUST apply exponential backoff with jitter starting at ~1 second (e.g., ~1s / ~2s / ~4s). Total retry wait MUST NOT exceed **~30 seconds of wall-clock time** per caller-visible request. If the budget is exhausted, the 429 MUST be surfaced to the caller as a typed rate-limit error (per FR-060).
- **FR-062**: The connector MUST never include credential material — specifically the OAuth client secret and the bearer access token — in error messages, logs, or telemetry it emits. The Account API-ID is NOT credential material for this purpose and MAY appear in structured log fields, error messages, tracing span attributes, and audit events.

### Key Entities

- **Cyclr Partner**: The top-level organisation / operator identity. Owns zero or more Accounts. A Partner-scoped credential can address Partner-level resources (Accounts, templates, Connectors catalog).
- **Cyclr Account**: A white-label tenant owned by a Partner. Has its own timezone, retention policies, webhook configuration, and its own set of installed Cycles and Connector installations. Addressed by its Cyclr identifier (often surfaced as an "API ID" on Account-scoped calls).
- **Cycle (workflow)**: An executable workflow installed inside an Account. Activation state (active / inactive) is independent of existence. Installed from a Cycle template.
- **Cycle Template**: A reusable blueprint from which Cycles are instantiated into Accounts. Lives at the Partner level.
- **Connector (third-party)**: A Cyclr-provided integration with an external SaaS (not to be confused with the `saas-connectors` library itself). Available from the Partner catalog and instantiated as a Connector installation inside an Account.
- **Connector Installation (AccountConnector)**: A specific instance of a third-party Connector inside a specific Account, along with its authorisation state. Creatable (install) and readable via the typed surface; not updatable or deletable via the typed surface in MVP.
- **Cycle Step**: A single node within an installed Cycle — represents one action or trigger against a third-party Connector. Read-only via the typed surface in MVP (full write access to step parameters and field mappings is deferred to pass-through).
- **Step Prerequisites**: A diagnostic view listing the parameters, field mappings, or authentications that a given Step requires but currently lacks, used by callers to diagnose why a Cycle cannot be activated. Read-only.
- **Step Parameter**: A single configurable input of a Step that is attached to the outbound third-party request as a header or URL component. Each Parameter has a `MappingType` (e.g., `StaticValue`, `ValueList`, `StepOutput`, `AccountVariable`) and a corresponding value. Readable and updatable via the typed surface.
- **Step Field Mapping**: A single configurable input of a Step that is attached to the outbound third-party request body. Same `MappingType` shape as Step Parameter; distinct Cyclr endpoint. Readable and updatable via the typed surface.
- **Pass-through Request**: An arbitrary authenticated HTTP call to a path under the configured Cyclr API domain, used for operations not yet covered by the typed surface.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator can create a brand-new white-label Account, install one Cycle template into it, and activate the Cycle end-to-end through the connector in under 2 minutes of wall-clock time, using only the connector's typed surface.
- **SC-002**: Onboarding a new customer Account (create Account + install up to 5 Cycles + activate them) completes without manually opening the Cyclr Console.
- **SC-003**: For every typed operation (list/read/create/update/delete on Accounts and Cycles, plus activate/deactivate and template-install), at least one automated test runs against mocked Cyclr responses and passes on every pull request.
- **SC-004**: For every typed operation, at least one credentialed integration test can be executed against a real Cyclr Partner sandbox and passes before release.
- **SC-005**: 100% of Cyclr API error responses encountered in practice translate to one of the library's typed error categories (no raw HTTP errors reach the gateway's callers).
- **SC-006**: The pass-through surface successfully proxies at least one Cyclr endpoint that is not part of the typed surface (proving the escape hatch works).
- **SC-007**: No credential material — specifically the OAuth client secret and the bearer access token — appears in any log line, error message, or telemetry emitted by the connector under any tested failure mode. Account API identifiers are identifiers (not credentials) and MAY appear, per FR-062 and the clarification in the Clarifications section.
- **SC-008**: Under normal onboarding traffic (one Account + five Cycle installs + five activations, back-to-back), the connector does not trip Cyclr's rate limits.
- **SC-009**: When a Cycle fails to activate because of incomplete configuration, the operator can determine *which* Step's parameter / field mapping / authentication is missing by reading Step Prerequisites through the connector, without consulting the Cyclr Console.
- **SC-010**: Installing a template whose required third-party Connectors are not yet installed in the Account succeeds end-to-end through the typed surface — the operator can install each missing Connector into the Account (FR-033), then install the template, without falling back to pass-through.
- **SC-011**: An MCP agent can reconfigure an installed Cycle by listing its Steps, listing each Step's parameters and field mappings, and updating the mapping values — ending with the Cycle in a state where reading its Prerequisites returns zero missing items — using only the typed surface.
- **SC-012**: The generated MCP tool surface downstream of this connector presents objects in the **six** coherent tool groups defined in FR-049 (Account lifecycle, Cycle control, Cycle diagnostics, Step configuration, Connector setup, Catalog), derived automatically from the object-name taxonomy. Every generated tool has a non-empty title (derived from `ObjectMetadata.DisplayName`), non-empty argument labels (derived from `FieldMetadata.DisplayName`), and typed arguments (derived from `FieldMetadata.ValueType`) — no unnamed arguments and no untyped arguments reach an MCP agent. (Long-form tool *descriptions* are out of scope at the connector level: Ampersand's `ObjectMetadata` does not yet carry a description field, per research.md §13.)

## Assumptions

- The gateway stores Cyclr credentials per connection (per `cxs2 solution_link_id`). Each connection binds to exactly one of the two providers (`cyclrPartner` or `cyclrAccount`) and therefore to exactly one scope for its lifetime. Switching scope, or addressing a different Account, requires a new connection (i.e., a new `solution_link_id` on the `cyclrAccount` provider).
- The Cyclr API domain (e.g., `api.cyclr.com`, `api.eu.cyclr.com`, `api.us2.cyclr.com`, `api.cyclr.uk`, or a private instance) is captured at connection creation. The connector does not attempt cross-region calls.
- Cycle *execution* (running Cycles and handling their step data in real-time) is owned by Cyclr itself. The connector operates on Cycle *lifecycle and configuration*, not on the runtime data flowing through Cycles.
- Real-time Cycle run monitoring (live transaction logs, step-by-step run telemetry) is out of scope for the first release; basic activation state is sufficient.
- White-label branding assets (custom domains, logos, CSS) are out of scope for the first release; the first release focuses on functional Account + Cycle lifecycle.
- Webhooks emitted by Cyclr on Cycle errors are consumed by downstream `fraios/saas-gateway` infrastructure, not by this connector. The connector's role is to configure the webhook URL on the Account, not to receive the callbacks.
- Cyclr's Data-on-Demand API (invoking an installed Connector's methods directly, without a Cycle) is reachable through `cyclrAccount`'s pass-through surface in MVP but is not given a typed read/write surface. Typed coverage is a post-MVP decision contingent on real demand.
- **Step-level configuration writes** (FR-035..039) ARE in MVP scope because the primary consumer is an MCP agent that must be able to reconfigure Cycles autonomously, not only install-and-run fixed templates. The mapping shape is expressed as a generic `MappingType` + `Value` (+ optional `SourceStepId` / `VariableName` etc. per type) — the connector does not attempt to validate that a given mapping value is accepted by the downstream Connector method (that's Cyclr's job). Agents discover valid mapping values iteratively via reads → Prerequisites → retry.
- **OAuth Connector authentication completion** (the browser-redirect sign-in-token flow for OAuth-backed third-party Connectors) is not part of this connector's typed surface. FR-033 creates the AccountConnector in "awaiting authorisation" state for OAuth Connectors; completing the OAuth dance belongs to the caller's UI layer and is reachable via pass-through if a server-side trigger is ever needed.
- The existing `saas-connectors` library conventions (component-based deep connector pattern, `BEST_PRACTICES.md`, `CONTRIBUTING.md` PR-per-capability rule) apply in full to this feature.
- Pull requests will be split per CONTRIBUTING's recipe — proxy / metadata / read / write / delete as separate PRs — so "ship the connector" is itself a multi-PR outcome.
- Existing upstream sync discipline (`DOWNSTREAM.md`) holds: this work is a proprietary fork addition and is the only expected divergence introduced by this feature.
- **MCP ergonomics are the downstream gateway's concern**, but this connector feeds them through its object taxonomy (FR-049) and metadata richness (FR-045..048). The Ampersand library does not have a first-class "tool group" or "progressive disclosure" concept; what we expose here as object names and field metadata is what the gateway's MCP layer turns into agent-facing tools. Naming and metadata discipline in this feature therefore has outsized impact on agent UX downstream.
