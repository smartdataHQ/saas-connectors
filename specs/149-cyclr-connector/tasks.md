---

description: "Task list for Cyclr Connector feature implementation"
---

# Tasks: Cyclr Connector

**Input**: Design documents from `/specs/149-cyclr-connector/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓

**Tests**: REQUIRED. Spec SC-003 mandates automated tests against mocked responses on every PR (Layer 1) and SC-004 mandates Layer-2 credentialed integration tests before release. `DOWNSTREAM.md` Layer 1 is mandatory for PR merge.

**Organization**: Tasks are grouped by user story. Phase 1 (Setup) adds the two proxy `ProviderInfo` files. Phase 2 (Foundational) builds the package skeletons and shared wiring that every deep capability requires. Phase 3+ deliver each user story as an independent, testable, deployable increment. Phase 2's proxy wiring incidentally satisfies most of User Story 4 (passthrough); US4 in Phase 6 is primarily verification.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1..US4)
- Exact file paths given for every task

## Path Conventions

Project is a Go library at repo root (`/Users/stefanbaxter/Development/saas-connectors`). Provider code lives in `providers/`; tests in `providers/<name>/*_test.go` (Layer 1) and `test/<Name>/` (Layer 2). Connector registry at `connector/new.go`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Introduce both providers at the ProviderInfo level so the catalog, registration, and downstream auth wiring can reference them. Satisfies PRs 1 and 2 of plan.md's sequence.

- [X] T001 Create `providers/cyclrPartner.go` with `const CyclrPartner Provider = "cyclrPartner"` and `init()` calling `SetInfo` per `contracts/cyclrPartner.md` (OAuth2 client credentials, `BaseURL: "https://{{.apiDomain}}"`, `TokenURL: "https://{{.apiDomain}}/oauth/token"`, `Support.Proxy: true`, `Metadata.Input` with `apiDomain`). No other Support flags set yet.
- [X] T002 [P] Create `providers/cyclrAccount.go` with `const CyclrAccount Provider = "cyclrAccount"` and `init()` calling `SetInfo` per `contracts/cyclrAccount.md` (OAuth2 client credentials, `ExplicitScopesRequired: true`, same BaseURL/TokenURL, `Support.Proxy: true`, `Metadata.Input` with `apiDomain` and `accountApiId`).
- [X] T003 [P] Run `make lint` and `go build ./...` after T001+T002 to confirm the two ProviderInfo entries compile and pass linters. (No dedicated file — validation step, recorded as a task for traceability.)

**Checkpoint**: Proxy passthrough is functional for both providers via the gateway's `generic.NewConnector` fallback. PR 1 and PR 2 from plan.md can merge.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish the deep-connector package skeletons, shared error/URL helpers, authenticated-transport middleware, and connector registry entries. Every user story's implementation code lives inside these packages.

**⚠️ CRITICAL**: User story phases cannot begin until this phase completes.

- [ ] T004 Create `providers/cyclrpartner/connector.go` with `type Connector struct { *components.Connector; common.RequireAuthenticatedClient; components.SchemaProvider; components.Reader; components.Writer; components.Deleter }`, `NewConnector(params)` calling `components.Initialize(providers.CyclrPartner, params, constructor)`, and a minimal `constructor` that wires only the error handler and endpoint registry (SchemaProvider/Reader/Writer/Deleter left as TODO comments to be filled per-story).
- [ ] T005 [P] Create `providers/cyclraccount/connector.go` with the same struct shape plus an `accountHeaderTransport` that implements `http.RoundTripper` and `Set`s `X-Cyclr-Account: {accountApiId}` on every outbound request (per research §9). Extract `accountApiId` from `ProviderContext` in `constructor`; wrap `base.HTTPClient().Client.Transport` before any handler wiring.
- [ ] T006 [P] Create `providers/cyclrpartner/errors.go` with `errorFormats` (single `FormatTemplate` matching `.NET`-style bodies: `Message`, optional `ExceptionMessage`, optional `ModelState`) and `statusCodeMapping` (401→`ErrAccessToken`, 403→`ErrRetryable` or `ErrCaller` per existing conventions, 404→`ErrObjectNotFound`, 422→`ErrBadRequest`, 429→`ErrRetryable` equivalent). Follow the `ResponseError.CombineErr` pattern from research §6. Include scope-mismatch detection symmetric to T007: if the `Message` field on a 401/403 contains a Cyclr-specific indicator that an **Account-scoped token** is being used against Partner-level endpoints, wrap with a clear scope-mismatch error per FR-005.
- [ ] T007 [P] Create `providers/cyclraccount/errors.go` with the same pattern as T006 plus scope-mismatch detection: if the `Message` field contains an indicator of wrong-scope credentials, wrap with a specific scope-mismatch error per FR-005.
- [ ] T008 [P] Create `providers/cyclrpartner/url.go` exposing `buildURL(params ...string) (*urlbuilder.URL, error)` that composes `BaseURL + apiVersion + path`. Export `const apiVersion = "v1.0"` at package scope.
- [ ] T009 [P] Create `providers/cyclraccount/url.go` exposing the same `buildURL` helper and `const apiVersion = "v1.0"`.
- [ ] T010 [P] Create `providers/cyclrpartner/utils.go` with minimal helpers (e.g., `isUUID(s string) bool` if used by handlers to short-circuit malformed `RecordId`).
- [ ] T011 [P] Create `providers/cyclraccount/utils.go` with minimal helpers.
- [ ] T012 Update `connector/new.go`: add imports for `providers/cyclrpartner` and `providers/cyclraccount`, add wrapper constructors `newCyclrPartnerConnector(p) (*cyclrpartner.Connector, error)` and `newCyclrAccountConnector(p) (*cyclraccount.Connector, error)`, add entries `providers.CyclrPartner: wrapper(newCyclrPartnerConnector)` and `providers.CyclrAccount: wrapper(newCyclrAccountConnector)` to the `connectorConstructors` map.
- [ ] T013 [P] Create `test/cyclrPartner/connector.go` — shared harness that loads creds via `credscanning.LoadPath(providers.CyclrPartner)` and returns a `*cyclrpartner.Connector`. Follow the pattern in `test/<provider>/connector.go` elsewhere in the repo.
- [ ] T014 [P] Create `test/cyclrAccount/connector.go` — shared harness; additionally validate that the creds JSON has a non-empty `accountApiId` in `metadata`, failing fast with a clear error if missing.
- [ ] T015 Run `make lint && go build ./...` to confirm foundational scaffolding compiles. Fix any issues surfaced.

**Checkpoint**: Foundation ready — all four user stories can begin in parallel (if staffed). Proxy passthrough was already satisfied by Phase 1.

---

## Phase 3: User Story 1 — White-label Account lifecycle (Priority: P1) 🎯 MVP

**Goal**: Partner operator can create, list, read, update, suspend, resume, and delete white-label Accounts via the `cyclrPartner` provider without touching the Cyclr Console.

**Independent Test**: Operator creates an Account via the connector, confirms visibility in the Cyclr Partner Console, updates its description, suspends it, resumes it, and deletes it — exercised automatically by `test/cyclrPartner/write/main.go` + `test/cyclrPartner/delete/main.go` in sequence.

**Corresponds to PRs**: 3 (Partner Metadata), 5 (Partner Read), 7 (Partner Write), 9 (Partner Delete, combined).

### Metadata

- [ ] T016 [US1] Create `providers/cyclrpartner/schemas.json` with an `accounts` object entry per `data-model.md` §Account (all fields, correct types, PascalCase preserved). Schema shape per `internal/staticschema/FieldMetadataMapV1`.
- [ ] T017 [US1] Create `providers/cyclrpartner/metadata.go` that loads `schemas.json` via `//go:embed` + `scrapper.NewMetadataFileManager[staticschema.FieldMetadataMapV1]`, exposes a package-level `schemas` variable, and wires `connector.SchemaProvider = schema.NewOpenAPISchemaProvider(connector.ProviderContext.Module(), schemas)` in `constructor` (extend T004's constructor).
- [ ] T018 [P] [US1] Create `providers/cyclrpartner/metadata_test.go` verifying `ListObjectMetadata([]string{"accounts"})` returns the expected fields and that unknown objects surface a typed error.

### Supported-operations registry

- [ ] T019 [US1] Create `providers/cyclrpartner/supports.go` with `func supportedOperations() components.EndpointRegistryInput` declaring `accounts` with `ReadSupport + WriteSupport + DeleteSupport`, plus `accounts:suspend` and `accounts:resume` with `WriteSupport` (per `contracts/cyclrPartner.md`).
- [ ] T020 [US1] Create `providers/cyclrpartner/objects.go` with `supportedObjectsByCreate`, `supportedObjectsByUpdate`, `supportedObjectsByDelete` sets for `accounts` (plus the two action names in the create set where relevant). Pattern mirrors `providers/capsule/objects.go`.

### Handlers (Read)

- [ ] T021 [US1] In `providers/cyclrpartner/handlers.go`, implement `buildReadRequest(ctx, params) (*http.Request, error)` for `accounts`: path is `/v1.0/accounts` for list and `/v1.0/accounts/{RecordId}` for by-id. Pagination: read `params.NextPage` (default "1"), set `page` and `per_page=50` query params. Wire into `reader.NewHTTPReader` factory in `constructor`.
- [ ] T022 [US1] In the same file, implement `parseReadResponse(ctx, params, resp) (*common.ReadResult, error)` for `accounts` using `common.ParseResult` with: record extractor pulling the array from the wrapped response (path confirmed at Layer 2 per research §12), `MakeMarshaledDataFunc` for field flattening, `NextPageFunc` derived from `Total-Pages` response header per research §5.
- [ ] T023 [P] [US1] Create `providers/cyclrpartner/read_test.go` using `mockserver.Conditional` to cover: list first page with next-page set, list last page with next-page empty, empty list (zero results), single by-id, 404 by-id, 429 with retry consumed successfully, 429 with retry exhausted.

### Handlers (Write)

- [ ] T024 [US1] In `providers/cyclrpartner/handlers.go`, implement `buildWriteRequest` for `accounts`: POST to `/v1.0/accounts` when `RecordId` empty, PUT to `/v1.0/accounts/{RecordId}` when present. Body is `params.RecordData` JSON-encoded (Cyclr consumes the shape directly per `data-model.md`).
- [ ] T025 [US1] Extend `buildWriteRequest` to handle synthetic object names `accounts:suspend` and `accounts:resume`: strip the suffix, POST to `/v1.0/accounts/{RecordId}/suspend` or `/v1.0/accounts/{RecordId}/resume` with empty body. (Same file as T024 — sequential.)
- [ ] T026 [US1] In the same file, implement `parseWriteResponse` that extracts `Id` from the response JSON (via `jsonquery.New(node).StringRequired("Id")`) and returns `&common.WriteResult{Success: true, RecordId: id}`. For suspend/resume, echo the incoming `RecordId` back.
- [ ] T027 [P] [US1] Create `providers/cyclrpartner/write_test.go` covering: create account returns `Id`, update account by `RecordId`, suspend round-trips `RecordId`, resume round-trips `RecordId`, validation error (422 with `ModelState` populated) surfaces typed error with field detail, auth error (401) surfaces typed error.

### Handlers (Delete)

- [ ] T028 [US1] In `providers/cyclrpartner/handlers.go`, implement `buildDeleteRequest` and `parseDeleteResponse` for `accounts` (DELETE `/v1.0/accounts/{RecordId}`, accept 204 No Content as success).
- [ ] T029 [P] [US1] Create `providers/cyclrpartner/delete_test.go` covering: delete success (204), delete refused by Cyclr with `Message` populated (surfaces typed error), delete of non-existent Account (404).

### Layer-2 integration entrypoints

- [ ] T030 [P] [US1] Create `test/cyclrPartner/metadata/main.go` — prints `ListObjectMetadata` result for `accounts`, `templates`, `connectors`. (Templates/connectors stubs for now; proper US3 coverage later.)
- [ ] T031 [P] [US1] Create `test/cyclrPartner/read/main.go` — lists accounts, reads the first by `Id`.
- [ ] T032 [P] [US1] Create `test/cyclrPartner/write/main.go` — creates a test Account named `SpecKit-US1-<timestamp>` with `Timezone: "UTC"`, updates its description, suspends it, resumes it. Prints the created `Id` for use by T033.
- [ ] T033 [P] [US1] Create `test/cyclrPartner/delete/main.go` — reads the most-recent `SpecKit-US1-*` Account and deletes it. Idempotent (no-op if no match).

**Checkpoint**: User Story 1 complete and independently verifiable. `go test ./providers/cyclrpartner/... -count=1 && make lint` green; `go run ./test/cyclrPartner/{metadata,read,write,delete}` successful against a real Cyclr Partner sandbox.

---

## Phase 4: User Story 2 — Operate Cycles inside a customer Account (Priority: P1)

**Goal**: Operator can install a Cycle from a template into an Account, activate, deactivate, list, read, and delete it via `cyclrAccount` — all without touching the Cyclr Console.

**Independent Test**: Using a pre-existing Account, the operator installs a known template, activates the Cycle, deactivates it, and deletes it — `test/cyclrAccount/write/main.go` + `test/cyclrAccount/delete/main.go` automate this.

**Corresponds to PRs**: 4 (Account Metadata), 6 (Account Read), 8 (Account Write), 9 (Account Delete, combined).

### Metadata

- [ ] T034 [US2] Create `providers/cyclraccount/schemas.json` with a `cycles` entry per `data-model.md` §Cycle (all fields, PascalCase preserved, includes nested `Connectors` array shape).
- [ ] T035 [US2] Create `providers/cyclraccount/metadata.go` — analog of T017 for `cyclraccount`, wiring `SchemaProvider` into `constructor`.
- [ ] T036 [P] [US2] Create `providers/cyclraccount/metadata_test.go` asserting schema for `cycles`.

### Supported-operations registry

- [ ] T037 [US2] Create `providers/cyclraccount/supports.go` declaring `cycles` with `ReadSupport + WriteSupport + DeleteSupport`, plus `cycles:activate` and `cycles:deactivate` with `WriteSupport`.
- [ ] T038 [US2] Create `providers/cyclraccount/objects.go` with create/update/delete sets for cycles and the action names.

### Handlers (Read)

- [ ] T039 [US2] In `providers/cyclraccount/handlers.go`, implement `buildReadRequest` for `cycles`: list is `/v1.0/cycles?page={N}&per_page=50`; by-id is `/v1.0/cycles/{RecordId}`.
- [ ] T040 [US2] In the same file, implement `parseReadResponse` for `cycles` with `ParseResult` + pagination-from-headers.
- [ ] T041 [P] [US2] Create `providers/cyclraccount/read_test.go` for `cycles`: list paginated, by-id, empty, 404, 429 retry. Verify `X-Cyclr-Account` header is attached to every outbound request by asserting on `mockcond.Header`.

### Handlers (Write)

- [ ] T042 [US2] In `providers/cyclraccount/handlers.go`, implement `buildWriteRequest` for `cycles` (the install-from-template case): when `ObjectName == "cycles"` and `params.RecordData["TemplateId"]` is set, POST to `/v1.0/templates/{TemplateId}/install` with empty body.
- [ ] T043 [US2] Extend `buildWriteRequest` to handle `cycles:activate`: PUT to `/v1.0/cycles/{RecordId}/activate` with body `{StartTime, Interval, RunOnce}` from `RecordData`. Validate `Interval` is in the closed allowed set (`1, 5, 15, 30, 60, 120, 180, 240, 360, 480, 720, 1440, 10080`) before dispatch; reject with a typed `ErrBadRequest` if not.
- [ ] T044 [US2] Extend `buildWriteRequest` to handle `cycles:deactivate`: PUT to `/v1.0/cycles/{RecordId}/deactivate` with empty body.
- [ ] T045 [US2] Implement `parseWriteResponse` for `cycles` and its action variants — extract `Id` on install-from-template, echo `RecordId` on activate/deactivate.
- [ ] T046 [P] [US2] Create `providers/cyclraccount/write_test.go` covering: install-from-template returns the new Cycle's `Id` (plus its `ErrorCount`/`WarningCount`), activate with valid `Interval` succeeds, activate with invalid `Interval` (e.g., 7) rejected client-side without HTTP call, activate on incompletely-configured Cycle surfaces Cyclr's refusal `Message`, deactivate succeeds.

### Handlers (Delete)

- [ ] T047 [US2] In `providers/cyclraccount/handlers.go`, implement `buildDeleteRequest` and `parseDeleteResponse` for `cycles` (DELETE `/v1.0/cycles/{RecordId}`).
- [ ] T048 [P] [US2] Create `providers/cyclraccount/delete_test.go` for `cycles` delete success and delete-refused paths.

### Cycle Step introspection (FR-026, FR-027, FR-028)

- [ ] T049 [US2] Extend `providers/cyclraccount/schemas.json` with `cycleSteps` entry (Id, Name, CycleId, ConnectorId, AccountConnectorId, MethodName, StepType, ErrorCount, WarningCount) per `data-model.md` §CycleStep. Also add a minimal `cycleSteps:prerequisites` entry.
- [ ] T050 [US2] Extend `providers/cyclraccount/supports.go` to declare `cycleSteps`, `cycleSteps:prerequisites`, and the parent-scoped pattern `cycles/*/steps` all with `ReadSupport`.
- [ ] T051 [US2] Extend `providers/cyclraccount/handlers.go` `buildReadRequest` to route:
  - `cycleSteps` with `RecordId` → `GET /v1.0/steps/{RecordId}`
  - `cycleSteps:prerequisites` with `RecordId` → `GET /v1.0/steps/{RecordId}/prerequisites`
  - object name matching `cycles/{cycleId}/steps` → `GET /v1.0/cycles/{cycleId}/steps?page=...&per_page=50`, parsing `cycleId` from the object name.
- [ ] T052 [US2] Extend `parseReadResponse` for the three new object shapes. For `cycleSteps` and the parent-scoped list, apply the credential-stripping heuristic on mapped parameter/field values before populating `Fields` (FR-028); preserve `Raw`. For `:prerequisites` surface Cyclr's response verbatim into `Fields`.
- [ ] T053 [P] [US2] Extend `providers/cyclraccount/read_test.go` with mock cases: list steps for a cycle (parent-scoped name), read single step by id, read step prerequisites (populated + empty array), credential-shaped field present in response is absent from `Fields` and present in `Raw`.

### Step configuration — parameters + field mappings (FR-035..039)

- [ ] T053a [US2] Extend `providers/cyclraccount/schemas.json` with `stepParameters` and `stepFieldMappings` entries (shared shape: `Id`, `StepId`, `Name`, `MappingType` with `Values` enum, `Value`, `SourceStepId`, `SourceFieldName`, `VariableName`, `AllowedValues`). Populate `DisplayName`, `ProviderType`, `ValueType`, `IsRequired`, `ReadOnly` on every field (FR-046). Populate `ReferenceTo` on `StepId`, `SourceStepId`, `VariableName` (→ account variables) per FR-048.
- [ ] T053b [US2] Extend `providers/cyclraccount/supports.go` to declare `stepParameters` and `stepFieldMappings` with `ReadSupport + WriteSupport`, plus parent-scoped globs `steps/*/parameters` and `steps/*/fieldmappings` with `ReadSupport`.
- [ ] T053c [US2] Extend `providers/cyclraccount/handlers.go` `buildReadRequest` to route:
  - `steps/{stepId}/parameters` → `GET /v1.0/steps/{stepId}/parameters?page=...&per_page=50`
  - `stepParameters` with `RecordId` → `GET /v1.0/steps/{stepId}/parameters/{parameterId}` using `StepId` from `RecordData.StepId` or compound `RecordId` fallback (Layer-2 confirmed)
  - `steps/{stepId}/fieldmappings` → `GET /v1.0/steps/{stepId}/fieldmappings` (same shape)
  - `stepFieldMappings` with `RecordId` → `GET /v1.0/steps/{stepId}/fieldmappings/{fieldId}`
- [ ] T053d [US2] Extend `parseReadResponse` for the four new read paths. Apply credential-stripping heuristic on `Value` (FR-039). Preserve `Raw`.
- [ ] T053e [US2] Extend `providers/cyclraccount/handlers.go` `buildWriteRequest` to route `stepParameters` updates: PUT `/v1.0/steps/{stepId}/parameters/{parameterId}`. Extract `StepId` from `RecordData`, place in URL path, forward only the mapping fields (`MappingType`, `Value`, `SourceStepId`, `SourceFieldName`, `VariableName`) in the body. Unknown `MappingType` values pass through uninterpreted (FR-037).
- [ ] T053f [US2] Extend `buildWriteRequest` for `stepFieldMappings` — analogous to T053e, PUT `/v1.0/steps/{stepId}/fieldmappings/{fieldId}`.
- [ ] T053g [US2] Extend `parseWriteResponse` for both — return `WriteResult{Success: true, RecordId: response.Id}`. Ensure `AuthValue`-style secrets in the submitted body do NOT appear in error context.
- [ ] T053h [P] [US2] Extend `providers/cyclraccount/read_test.go` with cases: list parameters for a step, read single parameter by step+id, parameter response with credential-shaped `Value` stripped in `Fields` and preserved in `Raw`, same four cases for field mappings.
- [ ] T053i [P] [US2] Extend `providers/cyclraccount/write_test.go` with cases: update parameter to `StaticValue` / `ValueList` / `StepOutput` / `AccountVariable`, 422 with `ModelState` populated, unknown `MappingType` accepted and forwarded, secret-shaped submitted values absent from error strings.

### AccountConnector install (FR-033, FR-034)

- [ ] T054 [US2] Extend `providers/cyclraccount/schemas.json` with `accountConnectors` (read fields + create-only fields: `Name`, `Description`, `AuthValue`, `ConnectorId`). Populate full metadata per FR-045..048.
- [ ] T055 [US2] Extend `providers/cyclraccount/supports.go` to declare `accountConnectors` with `ReadSupport + WriteSupport` (no delete in MVP).
- [ ] T056 [US2] Extend `providers/cyclraccount/handlers.go` `buildReadRequest` to route `accountConnectors` list (`GET /v1.0/account/connectors`) and by-id (`GET /v1.0/account/connectors/{RecordId}`). Apply credential-stripping heuristic to response fields.
- [ ] T057 [US2] Extend `providers/cyclraccount/handlers.go` `buildWriteRequest` to route `accountConnectors` install: extract `ConnectorId` from `RecordData`, `POST /v1.0/connectors/{ConnectorId}/install` with body `{Name, Description, AuthValue}`. Ensure `AuthValue` never appears in error context or log fields in this code path (FR-034).
- [ ] T058 [US2] Extend `parseWriteResponse` for `accountConnectors` — return `WriteResult{Success: true, RecordId: response.Id}`.
- [ ] T059 [P] [US2] Extend `providers/cyclraccount/write_test.go` with cases: install API-key Connector (Authenticated state), install OAuth Connector (AwaitingAuthentication state, `AuthValue` omitted), verify that a failing install's error message does NOT contain the supplied `AuthValue`.

### Layer-2 integration entrypoints

- [ ] T060 [P] [US2] Create `test/cyclrAccount/metadata/main.go`.
- [ ] T061 [P] [US2] Create `test/cyclrAccount/read/main.go` — lists cycles, reads the first by `Id`, lists its Steps via `cycles/{id}/steps`, reads one Step's prerequisites.
- [ ] T062 [P] [US2] Create `test/cyclrAccount/write/main.go` — ensures an API-key Connector is installed via `accountConnectors` (if missing), installs a template dependent on it, lists the resulting Cycle's Steps, picks one Step parameter, updates its mapping (`MappingType: StaticValue` with a test value), re-reads to confirm the update persisted, activates the resulting Cycle with `Interval: 60`, deactivates. This single flow exercises FR-020..038 against a real Cyclr sandbox.
- [ ] T063 [P] [US2] Create `test/cyclrAccount/delete/main.go` — deletes the Cycle created by T062.

**Checkpoint**: User Stories 1 AND 2 both independently functional. `go test ./providers/cyclr{partner,account}/... && make lint` green.

---

## Phase 5: User Story 3 — Catalog and connector visibility (Priority: P2)

**Goal**: Operator can list Cycle templates and third-party Connectors (at both scopes) and list the Connector installations present in an Account — without opening the Console to cross-reference IDs.

**Independent Test**: Operator calls `ListObjectMetadata`, reads `templates`, `connectors`, and `accountConnectors`, picks template + Connector IDs, and confirms those IDs work as inputs to User Story 2's install flow.

**Corresponds to PRs**: extends PR 3/4 (Metadata) and PR 5/6 (Read) with additional object types. Can ship as follow-up PRs after US1/US2 merge if sequencing demands.

> Note: `accountConnectors` (read + install) is part of User Story 2 (tasks T054–T059) because the install capability is a prerequisite to Story 2's template-install flow. Story 3 here covers only the remaining read-only discoverability surface: templates + catalog Connectors on the Partner side, plus the Account-side templates view.

### `cyclrPartner` extensions

- [ ] T064 [US3] Extend `providers/cyclrpartner/schemas.json` with `templates` and `connectors` object entries per `data-model.md` §Template and §Connector.
- [ ] T065 [US3] Update `providers/cyclrpartner/supports.go` to declare `templates` and `connectors` with `ReadSupport` only.
- [ ] T066 [US3] Extend `providers/cyclrpartner/handlers.go` `buildReadRequest` to route `templates` and `connectors` to `/v1.0/templates` and `/v1.0/connectors` respectively (plus by-id variants).
- [ ] T067 [US3] Extend `parseReadResponse` for `templates` and `connectors` — share pagination machinery with `accounts` via a small helper if the shape matches.
- [ ] T068 [P] [US3] Extend `providers/cyclrpartner/read_test.go` with mock cases for listing `templates` and `connectors` (first page, last page, empty).
- [ ] T069 [P] [US3] Extend `providers/cyclrpartner/metadata_test.go` with coverage for `templates` and `connectors` schemas.

### `cyclrAccount` extensions (templates-only; accountConnectors is in US2)

- [ ] T070 [US3] Extend `providers/cyclraccount/schemas.json` with a `templates` (read-only view) entry.
- [ ] T071 [US3] Update `providers/cyclraccount/supports.go` to declare `templates` with `ReadSupport` only.
- [ ] T072 [US3] Extend `providers/cyclraccount/handlers.go` `buildReadRequest` to route `templates` to `/v1.0/templates` (or Account-scoped variant if Layer-2 reveals a different path).
- [ ] T073 [P] [US3] Extend `providers/cyclraccount/read_test.go` with cases for listing and reading `templates` in Account scope.

### Layer-2 entrypoints

- [ ] T074 [P] [US3] Update `test/cyclrPartner/read/main.go` to additionally list `templates` and `connectors`.
- [ ] T075 [P] [US3] Update `test/cyclrAccount/read/main.go` to additionally list `templates` (accountConnectors was added in US2's T061).

**Checkpoint**: User Story 3 complete; catalog discovery enables all of US1 and US2's ID inputs without Console use.

---

## Phase 6: User Story 4 — Passthrough verification (Priority: P2)

**Goal**: Verify that pass-through works correctly at both scopes, including the critical FR-042 invariant that `cyclrAccount` pass-through cannot forge `X-Cyclr-Account`.

**Goal is mostly verification**: pass-through was enabled in Phase 1 (`Support.Proxy: true`); the `cyclrAccount` transport middleware was added in Phase 2 (T005). This phase exists to assert those behave correctly end-to-end.

**Independent Test**: Issue a raw HTTP call via the gateway's `/v1/proxy` endpoint to a Cyclr path not in the typed surface (e.g., `GET /v1.0/account/variables`) and receive Cyclr's response verbatim.

- [ ] T076 [US4] Add a test in `providers/cyclraccount/connector_test.go` (new file) asserting the `accountHeaderTransport` **overrides** any caller-set `X-Cyclr-Account` value (FR-042). Construct an `http.Request` with a forged header, pass it through the transport, confirm the header value equals the connection's `accountApiId` after round-trip.
- [ ] T077 [P] [US4] Add a test in `providers/cyclraccount/connector_test.go` asserting that requests to hosts **outside** the configured `apiDomain` are refused (FR-041). This may require a small refactor: if the transport currently forwards any host, add a host-allowlist check.
- [ ] T078 [P] [US4] Manual (or scripted) verification step using the dev proxy: `go run scripts/proxy/proxy.go` with `cyclrPartner` creds, issue `GET /v1.0/accounts` via `localhost:4444`, confirm a valid response. Repeat for `cyclrAccount` with an endpoint not in the typed surface (e.g., `/v1.0/account/variables` — Account Variables, explicit MVP-out-of-scope per Assumptions). Record results in quickstart.md `Smoke test flow` or a short PR-description paragraph.

**Checkpoint**: All four user stories complete and independently verifiable. Fork is now feature-complete for MVP; downstream gateway bump can proceed.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Repo-wide hygiene, real-API validation, docs alignment. None of these block any user story, but all must be green before the final downstream gateway bump.

- [ ] T078a [P] [US4] **FR-002 host-allowlist check.** Add a grep-driven test (or static check in `providers/cyclr{partner,account}/connector_test.go`) asserting that every `http.NewRequest` / `http.NewRequestWithContext` call inside the two packages derives its URL from `c.ProviderInfo().BaseURL` (i.e., starts with `https://{apiDomain}`). Closes the gap where FR-002's "refuse to call any host outside the configured domain" was only enforced for pass-through (T077) and not for typed handlers. Fail the test if any handler hard-codes a literal Cyclr host.
- [ ] T078b [P] [US4] **FR-062 credential-in-error check.** Add a test under `providers/cyclr{partner,account}/errors_test.go` that feeds an error response whose body happens to contain strings resembling credential material (`access_token`, `client_secret`, `Bearer xyz`) and asserts the wrapped error's `Error()` string does not contain them verbatim. Covers the broad FR-062 enforcement at the interpreter layer, complementing the narrower write-side checks in T059 and T053i.
- [ ] T079 [P] Run `make test-parallel` to flake-check the new mock-based tests. Any intermittent failure is a bug, not acceptable.
- [ ] T080 [P] Run the complete Layer-2 sequence (`test/cyclrPartner/*` then `test/cyclrAccount/*`) against a real Cyclr Partner sandbox. Record any deviations from research §12 open items (pagination header names, suspend/resume paths, delete cascade behaviour, template-listing path, prerequisites response shape, AccountConnector install response fields). File small follow-up PRs for each.
- [ ] T081 [P] Update `BEST_PRACTICES.md` if any Cyclr-specific pattern emerged that's worth promoting as a general best practice (e.g., transport-level header injection, credential-field stripping heuristic). Only if the pattern generalizes — do not pollute with provider-specifics.
- [ ] T082 Update `specs/149-cyclr-connector/quickstart.md` with any amendments uncovered during Layer-2 runs (actual pagination header names, any surprise Cyclr behaviours, how the `cycles/{id}/steps` pattern actually feels in practice).
- [ ] T083 Regenerate any `//go:generate` outputs touched by the new providers (if any — currently none expected, but run `go generate ./providers/...` to confirm).
- [ ] T084 Final `make lint && go test ./... -count=1 && go build ./...` pass across the whole repo to confirm no regression outside the new packages.
- [ ] T085 Prepare the downstream gateway bump documentation: short section in a tracking issue or PR body noting the `go get github.com/amp-labs/connectors@<sha>` command, the cxs2 credential fields now consumed (`apiDomain`, `accountApiId`, `clientId`, `clientSecret`, `scopes`), and the new object names the gateway's proto layer must accept (`accountConnectors`, `cycleSteps`, `cycleSteps:prerequisites`, `cycles/*/steps`, `stepParameters`, `stepFieldMappings`, `steps/*/parameters`, `steps/*/fieldmappings`).
- [ ] T086 **MCP metadata audit** (FR-045..048) — before merging PR 4/6/8, audit both `providers/cyclrpartner/schemas.json` and `providers/cyclraccount/schemas.json`: every object has `DisplayName`; every field has `DisplayName`, `ValueType`, `ProviderType`, `IsRequired` (set true/false, never null), `ReadOnly` (set for un-writable fields); every closed-set field (`Status`, `Interval`, `StepType`, `AuthenticationState`, `MappingType`) has `Values` populated; every lookup field (`TemplateId`, `CycleId`, `StepId`, `ConnectorId`, `AccountConnectorId`) has `ReferenceTo` populated. Produce a short compliance matrix in the PR description showing each object's completeness.
- [ ] T087 **Object-name taxonomy check** (FR-049) — grep across the two packages' `supports.go` files and assert every registered name fits the five-group taxonomy from FR-049. New names introduced outside the taxonomy fail the check and require either renaming or an explicit taxonomy extension.

---

## Dependencies & Execution Order

### Phase dependencies

- **Phase 1 (Setup)**: No dependencies — can start immediately.
- **Phase 2 (Foundational)**: Depends on Phase 1 completion — BLOCKS all user stories.
- **Phase 3 (US1)**: Depends on Phase 2 completion.
- **Phase 4 (US2)**: Depends on Phase 2 completion. **Independent of US1** — can run in parallel if staffed.
- **Phase 5 (US3)**: Depends on Phase 2. **Best sequenced after US1+US2** because it extends the same packages — touching the same files concurrently causes merge conflicts. If single-threaded, do US1 → US2 → US3.
- **Phase 6 (US4)**: Depends on Phase 2. Can run at any point after Phase 2 but benefits from US1+US2 being present to have real endpoints to pass-through to during verification.
- **Phase 7 (Polish)**: Depends on all user stories being complete.

### User story dependencies

- **US1 (P1)**: Depends on Foundational. Independent of US2/US3/US4.
- **US2 (P1)**: Depends on Foundational. Independent of US1. Files live in `providers/cyclraccount/` — zero overlap with US1's `providers/cyclrpartner/`.
- **US3 (P2)**: Extends `providers/cyclr{partner,account}/` files authored in US1 and US2. Best sequenced after both, to avoid merge thrash on `handlers.go` / `supports.go` / `schemas.json`.
- **US4 (P2)**: Largely a verification phase; new test files only. Independent.

### Within each user story

- Metadata (schema + SchemaProvider) before Handlers.
- `supports.go` before Handlers (registry must accept the object name before handler runs).
- `buildReadRequest` before `parseReadResponse` (not strictly, but one file — sequential).
- Layer-1 tests can be written in parallel with the implementation file they're testing (different files), but assertions can only be confirmed after implementation.
- Layer-2 main.go entrypoints come last — they require the compiled connector.

### Parallel opportunities

- T001 + T002 (two ProviderInfo files, different files).
- T004 + T005 + T006 + T007 + T008 + T009 + T010 + T011 — Phase 2 is almost entirely parallel; only T012 (registry edit) and T015 (verification) are sequential.
- T013 + T014 (test harnesses, different files).
- Layer-1 test files in each user story are [P] because each is a separate file.
- Layer-2 main.go entrypoints are [P] per story.
- US1 and US2 are fully parallel across their respective packages.
- US3 is the parallelism bottleneck — it edits the same files as US1/US2.

---

## Parallel Example: User Story 1 kick-off

After Phase 2 completes, a single developer can launch these in parallel:

```bash
# Tests authoring (different files)
Task: T018 "Create providers/cyclrpartner/metadata_test.go"
Task: T023 "Create providers/cyclrpartner/read_test.go"
Task: T027 "Create providers/cyclrpartner/write_test.go"
Task: T029 "Create providers/cyclrpartner/delete_test.go"
Task: T030-T033 "Create test/cyclrPartner/{metadata,read,write,delete}/main.go"
```

But the handler-authoring tasks — T021, T022, T024, T025, T026, and T028 — all edit `providers/cyclrpartner/handlers.go` and must therefore be sequential. T023, T027, and T029 are test-file tasks and can overlap with the handler work (different files).

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. Complete Phase 1 (Setup) → PRs 1 & 2 merge.
2. Complete Phase 2 (Foundational) → deep-connector scaffolding in place.
3. Complete Phase 3 (US1) → PRs 3, 5, 7, and the Partner half of PR 9 merge.
4. **STOP and VALIDATE**: `test/cyclrPartner/*` end-to-end against real sandbox.
5. Bump downstream `saas-gateway` with Partner-only capabilities. Ship.

At this point the gateway can manage white-label Accounts but not yet operate Cycles. Half the user value.

### Incremental Delivery (recommended)

1. Setup + Foundational → foundation ready.
2. Phase 3 (US1) → MVP: Account lifecycle. Bump downstream. Ship.
3. Phase 4 (US2) → add Cycle lifecycle. Bump downstream. Ship.
4. Phase 5 (US3) → add catalog discovery. Bump downstream. Ship.
5. Phase 6 (US4) → verify pass-through. No downstream change needed; Support.Proxy was already true from Phase 1.
6. Phase 7 (Polish) → final sweep, Layer-2 run, downstream bump with full feature set.

Each increment delivers user-visible value without breaking prior ones.

### Parallel Team Strategy

With two developers:

1. Dev A + Dev B together: Phase 1 + Phase 2.
2. Dev A: Phase 3 (US1, Partner side).
3. Dev B: Phase 4 (US2, Account side) — zero file overlap with Dev A.
4. Whoever finishes first: Phase 5 (US3) — solo to avoid merge conflicts.
5. Either: Phase 6 + Phase 7.

---

## Notes

- [P] = different files, no dependency on incomplete work.
- [Story] label ties each task to spec.md's user stories for traceability and independence.
- **Tests are mandatory** here (unlike the speckit default): spec SC-003 and DOWNSTREAM.md Layer 1 require Layer-1 mocks on every PR merge; spec SC-004 requires Layer-2 before release.
- Verify tests fail before implementing the corresponding handler (TDD discipline — not enforced by hook but good hygiene).
- Commit after each checklist item or small logical group. Each PR per plan.md's sequencing corresponds to a contiguous run of tasks (e.g., PR 3 = T016–T018; PR 5 = T021–T023; PR 7 = T024–T027).
- Stop at any phase checkpoint to validate the story independently before continuing.
- Avoid: editing the same file in [P]-labelled tasks (will conflict), cross-story dependencies on incomplete work, skipping the lint/test gate between phases.

---

## Task-count summary

| Phase | Tasks | Notes |
|---|---|---|
| Phase 1 — Setup | 3 | T001–T003. Delivers PRs 1 & 2 (proxy). |
| Phase 2 — Foundational | 12 | T004–T015. Delivers package skeletons + registration. |
| Phase 3 — US1 Account lifecycle | 18 | T016–T033. Delivers PRs 3, 5, 7, partial PR 9. |
| Phase 4 — US2 Cycle lifecycle + Steps + Step-config writes + Connector install | 39 | T034–T063 plus T053a..T053i. Delivers PRs 4, 6, 8, partial PR 9 — Cycle lifecycle, Step introspection + Prerequisites (FR-026..028), Step parameter / field-mapping read+write (FR-035..039), AccountConnector install (FR-033..034). |
| Phase 5 — US3 Catalog & visibility | 12 | T064–T075. Extends Phase 3 and Phase 4 with templates + Connector catalog reads. `accountConnectors` moved to US2 — no longer in this phase. |
| Phase 6 — US4 Pass-through verification | 5 | T076–T078 plus T078a (FR-002 host-allowlist check) and T078b (FR-062 credential-in-error check) — both added from the speckit-analyze coverage-gap findings. |
| Phase 7 — Polish | 9 | T079–T087. Cross-cutting — includes the FR-045..048 metadata audit (T086) and the FR-049 taxonomy check (T087). |
| **Total** | **98** | 96 numbered slots + 2 lettered additions (T078a, T078b). Counted including T053a..T053i (9 lettered sub-tasks). |

**MVP scope recommendation**: Phase 1 + Phase 2 + Phase 3 (33 tasks). Delivers FR-001..018, FR-050..062 (Partner scope), US1 end-to-end. Skipping US2 means no Cycle lifecycle — gateway can white-label Accounts but cannot operate them. Add Phase 4 (30 tasks) to reach the full P1 scope for both user stories.
