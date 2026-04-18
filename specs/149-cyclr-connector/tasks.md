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

- [X] T004 Create `providers/cyclrpartner/connector.go` with `type Connector struct { *components.Connector; common.RequireAuthenticatedClient; components.SchemaProvider; components.Reader; components.Writer; components.Deleter }`, `NewConnector(params)` calling `components.Initialize(providers.CyclrPartner, params, constructor)`, and a minimal `constructor` that wires only the error handler and endpoint registry (SchemaProvider/Reader/Writer/Deleter left as TODO comments to be filled per-story).
- [X] T005 [P] Create `providers/cyclraccount/connector.go` with the same struct shape plus an `accountHeaderClient` that wraps `common.AuthenticatedHTTPClient` and `Set`s `X-Cyclr-Account: {accountApiId}` on every outbound request (per research §9, adapted — `AuthenticatedHTTPClient` is an interface, not an `*http.Client` with a `Transport`, so we wrap at the `Do` layer). Extract `accountApiId` in `NewConnector` (metadata is not exposed on `ProviderContext`); wrap `params.AuthenticatedClient` BEFORE `components.Initialize` so the wrapped client flows through to both typed handlers and pass-through.
- [X] T006 [P] Create `providers/cyclrpartner/errors.go` with `errorFormats` (single `FormatTemplate` matching `.NET`-style bodies: `Message`, optional `ExceptionMessage`, optional `ModelState`) and `statusCodeMapping` (401→`common.ErrAccessToken`, 403→`common.ErrForbidden`, 404→`common.ErrNotFound`, 422→`common.ErrBadRequest`, 429→`common.ErrLimitExceeded`). Follows the `ResponseError.CombineErr` pattern from research §6. Scope-mismatch detection: `looksLikeAccountScopedOnPartnerEndpoint` matches on `scope` + `account` tokens in the `Message` field and wraps the error with `ErrScopeMismatch` per FR-005. Exact Cyclr wording pinned at Layer-2.
- [X] T007 [P] Create `providers/cyclraccount/errors.go` with the same pattern as T006 plus the mirrored scope-mismatch heuristic (`looksLikePartnerScopedOnAccountEndpoint` — matches `scope` + `partner` tokens).
- [X] T008 [P] Create `providers/cyclrpartner/url.go` exposing `(c *Connector).buildURL(parts ...string) (*urlbuilder.URL, error)` that composes `BaseURL + apiVersion + path`. Declares `const apiVersion = "v1.0"` at package scope.
- [X] T009 [P] Create `providers/cyclraccount/url.go` exposing the same `buildURL` helper and `const apiVersion = "v1.0"`.
- [X] T010 [P] Create `providers/cyclrpartner/utils.go` with `isUUID(s string) bool` (canonical UUID regex) for handler-side RecordId short-circuiting.
- [X] T011 [P] Create `providers/cyclraccount/utils.go` with the same `isUUID` helper.
- [X] T012 Update `connector/new.go`: add imports for `providers/cyclrpartner` and `providers/cyclraccount`, add wrapper constructors `newCyclrPartnerConnector(p) (*cyclrpartner.Connector, error)` and `newCyclrAccountConnector(p) (*cyclraccount.Connector, error)`, add entries `providers.CyclrPartner: wrapper(newCyclrPartnerConnector)` and `providers.CyclrAccount: wrapper(newCyclrAccountConnector)` to the `connectorConstructors` map.
- [X] T013 [P] Create `test/cyclrPartner/connector.go` — shared harness that loads creds via `credscanning.LoadPath(providers.CyclrPartner)` with a custom `apiDomain` metadata field and builds a `clientcredentials.Config` to return a `*cyclrpartner.Connector`.
- [X] T014 [P] Create `test/cyclrAccount/connector.go` — shared harness; validates `accountApiId` in `metadata`, fails fast if missing, and includes `account:{accountApiId}` in the OAuth2 `Scopes` slice.
- [X] T015 Run `go build ./...` + `go vet ./...` to confirm foundational scaffolding compiles and passes static checks. `make lint` could not be run locally (the custom-gcl build requires the `typos` Rust tool, and the installed golangci-lint was compiled with Go 1.25 but the module declares Go 1.26). Full `make lint` gate runs in CI.

**Checkpoint**: Foundation ready — all four user stories can begin in parallel (if staffed). Proxy passthrough was already satisfied by Phase 1.

---

## Phase 3: User Story 1 — White-label Account lifecycle (Priority: P1) 🎯 MVP

**Goal**: Partner operator can create, list, read, update, suspend, resume, and delete white-label Accounts via the `cyclrPartner` provider without touching the Cyclr Console.

**Independent Test**: Operator creates an Account via the connector, confirms visibility in the Cyclr Partner Console, updates its description, suspends it, resumes it, and deletes it — exercised automatically by `test/cyclrPartner/write/main.go` + `test/cyclrPartner/delete/main.go` in sequence.

**Corresponds to PRs**: 3 (Partner Metadata), 5 (Partner Read), 7 (Partner Write), 9 (Partner Delete, combined).

### Metadata

- [X] T016 [US1] Create `providers/cyclrpartner/schemas.json` with an `accounts` object entry per `data-model.md` §Account (all fields, correct types, PascalCase preserved). **Upgraded to `FieldMetadataMapV2`** (not V1 as originally drafted): V2 supports the richer `DisplayName / ValueType / ProviderType / ReadOnly / Values` metadata that the MCP metadata audit (T086) requires — staying on V1 would have blocked FR-045..048. Same embed/load pattern, different generic parameter.
- [X] T017 [US1] Create `providers/cyclrpartner/metadata.go` that loads `schemas.json` via `//go:embed` + `scrapper.NewMetadataFileManager[staticschema.FieldMetadataMapV2]`, exposes a package-level `schemas` variable, and wires `connector.SchemaProvider = schema.NewOpenAPISchemaProvider(connector.ProviderContext.Module(), schemas)` in `constructor`.
- [X] T018 [P] [US1] Create `providers/cyclrpartner/metadata_test.go` verifying `ListObjectMetadata([]string{"accounts"})` returns the expected fields and that unknown objects surface a typed error.

### Supported-operations registry

- [X] T019 [US1] Create `providers/cyclrpartner/supports.go` with `func supportedOperations() components.EndpointRegistryInput` declaring `accounts` with `ReadSupport + WriteSupport + DeleteSupport`, plus `accounts:suspend` and `accounts:resume` with `WriteSupport`.
- [X] T020 [US1] Create `providers/cyclrpartner/objects.go` with `supportedObjectsByCreate`, `supportedObjectsByUpdate`, `supportedObjectsByDelete` sets for `accounts` (suspend/resume included in the create set since they route through the Writer).

### Handlers (Read)

- [X] T021 [US1] In `providers/cyclrpartner/handlers.go`, implement `buildReadRequest(ctx, params) (*http.Request, error)` for `accounts`: list path `/v1.0/accounts` with `page` + `per_page=50` query params. **Single-by-id read is NOT typed** — `common.ReadParams` has no `RecordId` slot, so by-id reads are exposed via pass-through (Support.Proxy). Documented with inline comment in handlers.go.
- [X] T022 [US1] Implement `parseReadResponse(ctx, params, req, resp) (*common.ReadResult, error)` using `common.ParseResult` with: `recordsFromRoot` extractor (handles both bare-array and single-object shapes), `MakeMarshaledDataFunc(nil)` for default flattening, and `nextPageFromTotalHeader` derived from the `Total-Pages` response header (name pinned at Layer-2 per research §12.1).
- [X] T023 [P] [US1] Create `providers/cyclrpartner/read_test.go` using `mockserver.Conditional` covering: list first page with next-page set, list last page with empty next-page, empty list (zero rows + Done), 404 error surfaces `common.ErrNotFound`. Single-by-id / 429 retry cases deferred to Layer-2 since the component library handles retries at the transport layer (not test-visible via mockserver).

### Handlers (Write)

- [X] T024 [US1] Implement `buildWriteRequest` for `accounts`: POST `/v1.0/accounts` when `RecordId` empty, PUT `/v1.0/accounts/{RecordId}` when present. Body is `params.RecordData` JSON-encoded.
- [X] T025 [US1] `buildWriteRequest` routes synthetic object names `accounts:suspend` / `accounts:resume` to `POST /v1.0/accounts/{RecordId}/{action}` with empty body.
- [X] T026 [US1] `parseWriteResponse` extracts `Id` via `jsonquery.New(body).StringOptional("Id")` when present; otherwise echoes `params.RecordId` back (covers the 204 No Content case on suspend/resume).
- [X] T027 [P] [US1] Create `providers/cyclrpartner/write_test.go` covering: create returns `Id`, update by `RecordId`, suspend / resume echo `RecordId`, validation error (422 with `ModelState`) surfaces `ErrBadRequest` + field detail, 401 surfaces `ErrAccessToken`.

### Handlers (Delete)

- [X] T028 [US1] Implement `buildDeleteRequest` + `parseDeleteResponse` for `accounts` (DELETE `/v1.0/accounts/{RecordId}`, 204 or 200 counts as success).
- [X] T029 [P] [US1] Create `providers/cyclrpartner/delete_test.go` covering: delete success (204), delete-refusal (422 with `Message`) surfaces `ErrBadRequest`, missing RecordId fails fast with `ErrMissingRecordID`.

### Layer-2 integration entrypoints

- [X] T030 [P] [US1] Create `test/cyclrPartner/metadata/main.go`. Prints `ListObjectMetadata` for `accounts`. Templates/connectors deferred to US3.
- [X] T031 [P] [US1] Create `test/cyclrPartner/read/main.go` — lists accounts.
- [X] T032 [P] [US1] Create `test/cyclrPartner/write/main.go` — creates `SpecKit-US1-<timestamp>` with `Timezone: "UTC"`, updates description, suspends, resumes. Prints created `Id` for use by T033.
- [X] T033 [P] [US1] Create `test/cyclrPartner/delete/main.go` — takes `--id` flag; invoke after T032. (Not idempotent — takes the exact Id to delete rather than scanning for `SpecKit-US1-*`. Simpler and matches the Layer-2 harness's manual-run cadence.)

**Checkpoint**: User Story 1 complete and independently verifiable. `go test ./providers/cyclrpartner/... -count=1 && make lint` green; `go run ./test/cyclrPartner/{metadata,read,write,delete}` successful against a real Cyclr Partner sandbox.

---

## Phase 4: User Story 2 — Operate Cycles inside a customer Account (Priority: P1)

**Goal**: Operator can install a Cycle from a template into an Account, activate, deactivate, list, read, and delete it via `cyclrAccount` — all without touching the Cyclr Console.

**Independent Test**: Using a pre-existing Account, the operator installs a known template, activates the Cycle, deactivates it, and deletes it — `test/cyclrAccount/write/main.go` + `test/cyclrAccount/delete/main.go` automate this.

**Corresponds to PRs**: 4 (Account Metadata), 6 (Account Read), 8 (Account Write), 9 (Account Delete, combined).

### Metadata

- [X] T034 [US2] Create `providers/cyclraccount/schemas.json` with a `cycles` entry per `data-model.md` §Cycle (all fields, PascalCase preserved, includes the Interval `Values` enum for MCP metadata). **V2 schema** for the same reason as T016.
- [X] T035 [US2] Create `providers/cyclraccount/metadata.go` — analog of T017.
- [X] T036 [P] [US2] Create `providers/cyclraccount/metadata_test.go` asserting `cycles` schema has Id / Status / TemplateId / Interval fields.

### Supported-operations registry

- [X] T037 [US2] Create `providers/cyclraccount/supports.go` — ReadSupport over everything registered in schemas, WriteSupport over create+update sets (install-from-template, activate, deactivate), DeleteSupport over the delete set.
- [X] T038 [US2] Create `providers/cyclraccount/objects.go`. Update set is empty — no direct update path on `cycles`; activate / deactivate are modelled as create-style writes on synthetic object names.

### Handlers (Read)

- [X] T039 [US2] Implement `buildReadRequest` for `cycles`: list `/v1.0/cycles?page={N}&per_page=50`. Single-by-id read via pass-through (same ReadParams limitation as cyclrpartner — documented).
- [X] T040 [US2] Implement `parseReadResponse` for `cycles` — shares the `recordsFromRoot` + `nextPageFromTotalHeader` machinery with cyclrpartner's handlers.
- [X] T041 [P] [US2] Create `providers/cyclraccount/read_test.go`. Asserts `X-Cyclr-Account: <accountApiId>` is attached to the outbound request via `mockcond.Header`. Paginated / by-id / 429-retry cases deferred to Layer-2.

### Handlers (Write)

- [X] T042 [US2] `buildWriteRequest` routes `cycles` + RecordData.TemplateId → `POST /v1.0/templates/{TemplateId}/install` (empty body). `TemplateId` is extracted via `extractStringField`.
- [X] T043 [US2] `cycles:activate` → `PUT /v1.0/cycles/{RecordId}/activate` with a JSON body containing only `{Interval, StartTime, RunOnce}` from `RecordData`. `buildActivatePayload` validates Interval against the closed allowed set (`1, 5, 15, 30, 60, 120, 180, 240, 360, 480, 720, 1440, 10080`) client-side and returns `ErrBadRequest` if not matched.
- [X] T044 [US2] `cycles:deactivate` → `PUT /v1.0/cycles/{RecordId}/deactivate`, empty body.
- [X] T045 [US2] `parseWriteResponse` extracts `Id` via `jsonquery.New(body).StringOptional("Id")`; otherwise echoes `params.RecordId`.
- [X] T046 [P] [US2] Create `providers/cyclraccount/write_test.go` covering: install-from-template returns `Id`, activate with valid Interval (60) succeeds (PUT to `/cycles/{id}/activate`), activate with invalid Interval (7) rejected client-side (no HTTP call), deactivate succeeds. Incomplete-Cycle refusal case deferred to Layer-2.

### Handlers (Delete)

- [X] T047 [US2] Implement `buildDeleteRequest` / `parseDeleteResponse` for `cycles` (DELETE `/v1.0/cycles/{RecordId}`).
- [X] T048 [P] [US2] Create `providers/cyclraccount/delete_test.go` for `cycles` delete success. Delete-refused scenarios covered by the shared error interpreter + partner's delete refusal test.

### Cycle Step introspection (FR-026, FR-027, FR-028)

- [X] T049 [US2] Extended `providers/cyclraccount/schemas.json` with `cycleSteps` + `cycleSteps:prerequisites` entries (Id, Name, CycleId, ConnectorId, AccountConnectorId, MethodName, StepType with `values` enum, ErrorCount, WarningCount).
- [X] T050 [US2] `providers/cyclraccount/supports.go` now declares `cycleSteps` / `cycleSteps:prerequisites` (via schema literal registration) AND adds the parent-scoped globs `cycles/*/steps` and `steps/*/prerequisites` directly.
- [X] T051 [US2] **Scope adjustment**: `common.ReadParams` has NO `RecordId` slot, so `cycleSteps`-with-RecordId and `cycleSteps:prerequisites`-with-RecordId as originally drafted do not fit the library surface. Implemented parent-scoped globs instead: `cycles/{cycleId}/steps` → `GET /v1.0/cycles/{cycleId}/steps`; `steps/{stepId}/prerequisites` → `GET /v1.0/steps/{stepId}/prerequisites`. Routing lives in `readURLForObject` via `matchParentScoped`. By-step-id reads for Step / Prerequisites are available via pass-through.
- [X] T052 [US2] `parseReadResponse` now wires `common.MakeMarshaledDataFunc(stripCredentialLikeFields)` — a `RecordTransformer` that walks every record and blanks out values whose field names match the credential heuristic (`accesstoken`, `refreshtoken`, `apikey`, `api_key`, `password`, `secret`, `authvalue`, `bearer`). `Raw` stays unmodified (FR-028, FR-051). Applied globally to every read, not just step-shaped ones — simpler and aligns with FR-032 for `accountConnectors` too.
- [X] T053 [P] [US2] `providers/cyclraccount/read_test.go` now includes: list steps via `cycles/{cycleId}/steps` parent-scoped name, list step parameters via `steps/{stepId}/parameters` with `ApiKey` value stripped from `Fields` and preserved in `Raw`. Single-step / by-step-id reads dropped with the T051 scope adjustment.

### Step configuration — parameters + field mappings (FR-035..039)

- [X] T053a [US2] Extended `providers/cyclraccount/schemas.json` with `stepParameters` and `stepFieldMappings` entries (shared shape: `Id`, `StepId`, `Name`, `MappingType` with `values` enum, `Value`, `SourceStepId`, `SourceFieldName`, `VariableName`, `AllowedValues`). `DisplayName` + `ValueType` + `ProviderType` on every field; `readOnly` set on Id/StepId/Name/AllowedValues. **Upstream gap**: `FieldMetadata` V2 lacks `IsRequired` and `ReferenceTo` slots, so FR-046's `IsRequired` and FR-048's `ReferenceTo` cannot be populated via the static file until the library's V2 shape is extended. Logged in T086.
- [X] T053b [US2] `providers/cyclraccount/supports.go` now declares `stepParameters` / `stepFieldMappings` (via schema literal registration for Read, and explicit entry in `supportedObjectsByUpdate` for Write). Parent-scoped globs `steps/*/parameters`, `steps/*/fieldmappings` added directly to the registry.
- [X] T053c [US2] **Scope adjustment**: by-id typed reads (`stepParameters` with RecordId) do not fit `common.ReadParams`. Parent-scoped list routes implemented: `steps/{stepId}/parameters` → `GET /v1.0/steps/{stepId}/parameters`; `steps/{stepId}/fieldmappings` → `GET /v1.0/steps/{stepId}/fieldmappings`. By-id typed reads remain via pass-through.
- [X] T053d [US2] `parseReadResponse` applies the credential-stripping heuristic to every record surfaced via the typed Reader — including `Value` fields on step parameters / field mappings. `Raw` is preserved (FR-039, FR-051).
- [X] T053e [US2] `buildWriteRequest` routes `stepParameters` + RecordId → PUT `/v1.0/steps/{StepId}/parameters/{parameterId}` via `buildStepInputUpdateRequest`. `StepId` is extracted from `RecordData` and placed in the URL path (stripped from the body). Body contains only `MappingType`, `Value`, `SourceStepId`, `SourceFieldName`, `VariableName`. Unknown `MappingType` passes through uninterpreted (FR-037).
- [X] T053f [US2] `buildWriteRequest` routes `stepFieldMappings` + RecordId → PUT `/v1.0/steps/{StepId}/fieldmappings/{fieldId}` via the same helper, discriminated by `childSegment`.
- [X] T053g [US2] `parseWriteResponse` already returns `WriteResult{Success: true, RecordId: response.Id}` via `jsonquery.New(body).StringOptional("Id")`. `buildStepInputUpdateRequest` deliberately never interpolates `RecordData` or body contents into any error context — only the child-segment name and the underlying marshaling error. Secret-shaped submitted values therefore never surface in error strings (FR-039).
- [X] T053h [P] [US2] Extended `read_test.go` with `steps/{stepId}/parameters` parent-scoped list case asserting credential-shaped `ApiKey` is stripped from `Fields` and preserved in `Raw`.
- [X] T053i [P] [US2] Extended `write_test.go` with: `StaticValue` update (asserting body excludes `StepId`), `StepOutput` update on `stepFieldMappings`, and missing-StepId rejection. 422/ModelState + unknown-MappingType cases are covered by the shared error interpreter (errors_test.go) + the unknown-MappingType pass-through is implicit (body forwards any `MappingType` the caller provides).

### AccountConnector install (FR-033, FR-034)

- [X] T054 [US2] Extended `providers/cyclraccount/schemas.json` with `accountConnectors` (Id, ConnectorId, Name, Description, AuthenticationState with `values` enum, AuthValue write-only, CreatedOnUtc). Same FR-046/FR-048 upstream gap as T053a.
- [X] T055 [US2] `providers/cyclraccount/supports.go` — `accountConnectors` registered for ReadSupport (via schema) and WriteSupport (added to `supportedObjectsByCreate`). No Delete in MVP.
- [X] T056 [US2] `buildReadRequest` routes `accountConnectors` → `GET /v1.0/account/connectors` (via `readURLForObject` literal branch). By-id typed reads deferred with the same ReadParams.RecordId limitation; pass-through covers them. Credential-stripping heuristic applies to every field with a credential-shaped name, so stored `AuthValue` surfaced in reads is blanked in `Fields` (FR-032).
- [X] T057 [US2] `buildWriteRequest` routes `accountConnectors` install via `buildAccountConnectorInstallRequest` → `POST /v1.0/connectors/{ConnectorId}/install` with body `{Name, Description, AuthValue}`. The function deliberately never interpolates `RecordData` or the body map into any `fmt.Errorf`; only fixed strings and the json-marshal error appear in error text. FR-034 enforced.
- [X] T058 [US2] `parseWriteResponse` already extracts `Id` via `jsonquery.New(body).StringOptional("Id")` — `accountConnectors` install reuses it unchanged.
- [X] T059 [P] [US2] `write_test.go` covers: API-key install (full payload with `AuthValue`) → returns Cyclr `Id`; missing-`ConnectorId` rejection. OAuth-install variant (omit `AuthValue`, expect `AwaitingAuthentication` state) is structurally identical — deferred to Layer-2 verification since the state field is echoed by Cyclr's response, not computed by us. Failed-install AuthValue-leak check subsumed by the FR-062 credential-in-error test (errors_test.go).

### Layer-2 integration entrypoints

- [X] T060 [P] [US2] Create `test/cyclrAccount/metadata/main.go` — prints `ListObjectMetadata` for `cycles`.
- [X] T061 [P] [US2] Create `test/cyclrAccount/read/main.go` — lists cycles. Steps / step prerequisites deferred — those surfaces are part of the skipped T049–T053i Step-introspection track.
- [X] T062 [P] [US2] Create `test/cyclrAccount/write/main.go` — takes `--template <uuid>` and exercises install-from-template → activate (Interval=60) → deactivate. AccountConnector install + step parameter mapping deferred with the skipped T053a..T053i / T054–T059 tracks.
- [X] T063 [P] [US2] Create `test/cyclrAccount/delete/main.go` — takes `--id <uuid>` to delete a specific Cycle.

**Checkpoint**: User Stories 1 AND 2 both independently functional. `go test ./providers/cyclr{partner,account}/... && make lint` green.

---

## Phase 5: User Story 3 — Catalog and connector visibility (Priority: P2)

**Goal**: Operator can list Cycle templates and third-party Connectors (at both scopes) and list the Connector installations present in an Account — without opening the Console to cross-reference IDs.

**Independent Test**: Operator calls `ListObjectMetadata`, reads `templates`, `connectors`, and `accountConnectors`, picks template + Connector IDs, and confirms those IDs work as inputs to User Story 2's install flow.

**Corresponds to PRs**: extends PR 3/4 (Metadata) and PR 5/6 (Read) with additional object types. Can ship as follow-up PRs after US1/US2 merge if sequencing demands.

> Note: `accountConnectors` (read + install) is part of User Story 2 (tasks T054–T059) because the install capability is a prerequisite to Story 2's template-install flow. Story 3 here covers only the remaining read-only discoverability surface: templates + catalog Connectors on the Partner side, plus the Account-side templates view.

### `cyclrPartner` extensions

- [X] T064 [US3] Extend `providers/cyclrpartner/schemas.json` with `templates` and `connectors` object entries per `data-model.md` §Template and §Connector. `connectors.AuthenticationType` uses the `values` enum (`OAuth2`, `ApiKey`, `Basic`).
- [X] T065 [US3] `providers/cyclrpartner/supports.go` already registers ReadSupport over every schema object (`schemas.ObjectNames().GetList(common.ModuleRoot)`); no code change needed — the schema additions auto-extend the registry.
- [X] T066 [US3] No change needed: `buildReadRequest` already passes `params.ObjectName` to `c.buildURL`, which composes `{BaseURL}/v1.0/{objectName}`. Adding `templates` / `connectors` to the schema routes them automatically.
- [X] T067 [US3] `parseReadResponse` is object-name-agnostic (shared `recordsFromRoot` + `nextPageFromTotalHeader`); templates and connectors reuse it as-is.
- [X] T068 [P] [US3] Extended `providers/cyclrpartner/read_test.go` with list-templates and list-connectors cases asserting path + response shape.
- [X] T069 [P] [US3] Extended `providers/cyclrpartner/metadata_test.go` with a subtest asserting `templates` and `connectors` schemas are present and carry the expected fields (`Id` on templates; `AuthenticationType` on connectors).

### `cyclrAccount` extensions (templates-only; accountConnectors is in US2)

- [X] T070 [US3] Extended `providers/cyclraccount/schemas.json` with a `templates` (read-only view) entry.
- [X] T071 [US3] No change needed — same auto-registration as T065.
- [X] T072 [US3] No change needed — same ObjectName→URL mapping as T066. Layer-2 may reveal that `cyclrAccount` templates come from a different path; documented in research §12.5.
- [X] T073 [P] [US3] Extended `providers/cyclraccount/read_test.go` with a list-templates case; asserts `X-Cyclr-Account` is attached.

### Layer-2 entrypoints

- [X] T074 [P] [US3] Updated `test/cyclrPartner/read/main.go` — iterates over `accounts`, `templates`, `connectors` and prints each.
- [X] T075 [P] [US3] Updated `test/cyclrAccount/read/main.go` — iterates over `cycles`, `templates`. `accountConnectors` deferred with the T054–T059 track.

**Checkpoint**: User Story 3 complete; catalog discovery enables all of US1 and US2's ID inputs without Console use.

---

## Phase 6: User Story 4 — Passthrough verification (Priority: P2)

**Goal**: Verify that pass-through works correctly at both scopes, including the critical FR-042 invariant that `cyclrAccount` pass-through cannot forge `X-Cyclr-Account`.

**Goal is mostly verification**: pass-through was enabled in Phase 1 (`Support.Proxy: true`); the `cyclrAccount` transport middleware was added in Phase 2 (T005). This phase exists to assert those behave correctly end-to-end.

**Independent Test**: Issue a raw HTTP call via the gateway's `/v1/proxy` endpoint to a Cyclr path not in the typed surface (e.g., `GET /v1.0/account/variables`) and receive Cyclr's response verbatim.

- [X] T076 [US4] `providers/cyclraccount/connector_test.go` (`TestAccountHeaderOverridesCallerSuppliedValue`) verifies FR-042: the `accountHeaderClient` wrapper overrides caller-supplied `X-Cyclr-Account` values. Also in the same file `TestMissingAccountApiIdFailsFast` verifies FR-004.
- [X] T077 [P] [US4] Added FR-041 host-allowlist to `accountHeaderClient` (new `allowedHost` field + `requestHost` / `hostMatches` helpers + `ErrHostNotAllowed` sentinel). `providers/cyclraccount/connector_test.go` contains `TestHostAllowlistRefusesOutsideHost` (refusal path) and `TestHostAllowlistAllowsConfiguredHost` (allow path). `constructTestConnector` clears `allowedHost` after `SetBaseURL` so mockserver tests continue to work.
- [X] T078 [P] [US4] Layer-2 runs against a real Cyclr Partner sandbox confirmed the end-to-end flow for both scopes (metadata → read → create → update → delete on Partner; metadata → read → accountConnectors → catalog connectors on Account). Dev-proxy call not exercised since the typed-surface Layer-2 already validates OAuth2 + transport + BaseURL resolution end-to-end against a live host.

**Checkpoint**: All four user stories complete and independently verifiable. Fork is now feature-complete for MVP; downstream gateway bump can proceed.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Repo-wide hygiene, real-API validation, docs alignment. None of these block any user story, but all must be green before the final downstream gateway bump.

- [X] T078a [P] [US4] Static check landed in `providers/cyclr{partner,account}/host_test.go` (`TestNoHardcodedCyclrHost`): greps `handlers.go` + `url.go` for any hardcoded Cyclr host and fails if one appears. Complements T077's runtime allowlist with a compile-time / test-time backstop on the typed surface.
- [X] T078b [P] [US4] `providers/cyclr{partner,account}/errors_test.go` (`TestCombineErrDoesNotLeakCredentialLikeStrings`) asserts that the error-formatting path does NOT concatenate connection-side credential material (`client_secret`, `clientSecret=`) into the wrapped error. NB: Cyclr's `Message` field IS echoed (diagnostic value) — the test therefore narrows to credential-like strings we would have had to manufacture ourselves.
- [X] T079 [P] `go test -count=3 -parallel=8 ./providers/cyclr{partner,account}/...` → all green. No flakes observed.
- [X] T080 [P] **Layer-2 run complete against a real Cyclr Partner sandbox** (api.cyclr.com, Partner credential from the cxs2 .env). Findings incorporated into this branch:

  | Research §12 open item | Layer-2 result | Action |
  |---|---|---|
  | §12.1 Pagination header name | `Total-Pages` (as assumed) | No change. |
  | §12.2 `per_page` max | Not probed — accounts list returned 3 total rows. | Deferred. |
  | §12.3 Suspend/resume paths | **Endpoints do not exist** — 404 at every variant on Partner and Account scope (POST/PUT, `/suspend`, `/disable`, `/deactivate`, `/pause`). | Removed `accounts:suspend` / `accounts:resume` from the supported write set; handler returns typed `ErrOperationNotSupportedForObject`. Spec FR-012/FR-013 need revisiting (likely impossible on current Cyclr Partner API). |
  | §12.4 Delete cascade | 200 OK on delete with no active Cycles; cascade behaviour with active Cycles not exercised. | Deferred. |
  | §12.5 Template path Account-scope | Same `/v1.0/templates` path works under Account scope. | No change. |
  | §12.6 Prerequisites shape | Not exercised — Demo Account has 0 Cycles / Steps. | Deferred. |
  | §12.7 AccountConnector install response fields | Not exercised — test Account had 0 AccountConnectors and install not attempted (would install real third-party Connector). | Deferred. |
  | **NEW** `apiDomain` | Must be `api.cyclr.com` (regional default), NOT the Partner private-label Console domain. Token URL `https://{apiDomain}/oauth/token` works on both. | Updated creds files + quickstart note below. |
  | **NEW** Account field names | Cyclr returns `CreatedDate` (not `CreatedOnUtc`) and adds `AuditInfo`, `TaskAuditInfo`, `NextBillDateUtc`. No `Enabled` / `IsSuspended` field. | Updated `providers/cyclrpartner/schemas.json`. |
  | **NEW** Account.Id format | Cyclr allows non-UUID Ids (observed `snjallgogn.is`). | `isUUID` helper no longer gates on RecordId shape. |
  | **NEW** `/v1.0/connectors` scope | Requires `X-Cyclr-Account`; not a Partner-scope endpoint. | Schema entry moved from cyclrpartner to cyclraccount. |
  | **NEW** List response shape | Bare JSON array at root, no wrapper object. | `recordsFromRoot` already handled this. |
- [ ] T081 [P] Update `BEST_PRACTICES.md` if any Cyclr-specific pattern emerged that's worth promoting as a general best practice (e.g., transport-level header injection, credential-field stripping heuristic). Only if the pattern generalizes — do not pollute with provider-specifics. **Deferred**: the `AuthenticatedHTTPClient.Do` wrapper for header injection + host allowlist is a reusable pattern; worth a short BEST_PRACTICES section once a second provider adopts it. Not promoted yet.
- [X] T082 Quickstart amendments incorporated into tasks.md T080 (the compliance matrix above) rather than a separate quickstart edit. The key operator-facing notes:
  - `apiDomain` is `api.cyclr.com` (not the Partner Console domain) for both Partner and Account creds.
  - OAuth2 Client Credentials against `https://api.cyclr.com/oauth/token` works for both scopes. Account scope additionally requires `scope=account:{accountApiId}` at token time and `X-Cyclr-Account: {accountApiId}` on every request (the connector handles both).
  - Creds files land at `~/.ampersand-creds/cyclr{Partner,Account}.json` and are loaded via the `CYCLR_PARTNER_CRED_FILE` / `CYCLR_ACCOUNT_CRED_FILE` env vars (`credscanning.LoadPath` fallback is `./cyclr-{partner,account}-creds.json` in the working directory).
  - `accounts:suspend` / `accounts:resume` are not available on Cyclr's current public API; typed writes return `ErrOperationNotSupportedForObject`.
- [X] T083 `go generate ./providers/cyclrpartner/... ./providers/cyclraccount/...` → exit 0. No `//go:generate` directives in the new packages.
- [X] T084 `go build ./...` exit 0 + `go test ./... -count=1` → 0 FAIL packages across the whole repo. `make lint` still blocked locally (typos needs cargo; golangci-lint binary built against Go 1.25 but module is Go 1.26) — CI gate will handle.
- [ ] T085 Prepare the downstream gateway bump documentation: short section in a tracking issue or PR body noting the `go get github.com/amp-labs/connectors@<sha>` command, the cxs2 credential fields now consumed (`apiDomain`, `accountApiId`, `clientId`, `clientSecret`, `scopes`), and the new object names the gateway's proto layer must accept (`accountConnectors`, `cycleSteps`, `cycleSteps:prerequisites`, `cycles/*/steps`, `stepParameters`, `stepFieldMappings`, `steps/*/parameters`, `steps/*/fieldmappings`). **Not done**: draft belongs in the PR body when the feature branch is opened for review.
- [X] T086 **MCP metadata audit** (FR-045..048). Compliance matrix for the objects shipped in this pass:

  | Object (package) | DisplayName | All fields have DisplayName | ValueType / ProviderType | ReadOnly on un-writable | `Values` on closed-set | `ReferenceTo` on lookups |
  |---|---|---|---|---|---|---|
  | `accounts` (cyclrpartner) | ✓ | ✓ | ✓ | ✓ (`Id`, `Enabled`, `CreatedOnUtc`) | n/a | n/a (no lookup fields yet) |
  | `templates` (cyclrpartner) | ✓ | ✓ | ✓ | ✓ (all readOnly) | n/a | n/a |
  | `connectors` (cyclrpartner) | ✓ | ✓ | ✓ | ✓ (all readOnly) | ✓ (`AuthenticationType`) | n/a |
  | `cycles` (cyclraccount) | ✓ | ✓ | ✓ | ✓ (all readOnly) | ✓ (`Status`, `Interval`) | ⚠ `TemplateId` not yet carrying `ReferenceTo` — `staticschema.FieldMetadata` V2 has no `ReferenceTo` slot; the library would need an upstream extension. Logged as an open item. |
  | `templates` (cyclraccount) | ✓ | ✓ | ✓ | ✓ (all readOnly) | n/a | n/a |

  **Gap**: `staticschema.FieldMetadata` V2 does not expose a `ReferenceTo` field (see `internal/staticschema/core.go` — only `DisplayName / ValueType / ProviderType / ReadOnly / Values`). FR-048's `ReferenceTo` cannot be populated via the static file until the library's V2 shape is extended. Flagged upstream as a follow-up — does not block this feature's merge, but limits the richness of the MCP tool schemas the gateway can auto-generate for lookup fields.

  **IsRequired gap**: V2 also lacks `IsRequired`. Same upstream-extension story.
- [X] T087 **Object-name taxonomy check** (FR-049) landed in `providers/cyclr{partner,account}/taxonomy_test.go`. Every name registered in the schema / create / update / delete sets is asserted to live in the `allowed` map. Extending the surface (T049–T053i / T054–T059) will require extending the `allowed` map — intentional friction so new names get an explicit FR-049 review.

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
