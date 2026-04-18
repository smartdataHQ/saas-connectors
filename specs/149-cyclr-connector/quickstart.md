# Quickstart: Cyclr Connector

How to validate this feature locally, at each test layer.

---

## Prerequisites

1. Go 1.25+ and working `make install/dev` + `make custom-gcl`. See `DOWNSTREAM.md` §"Getting started on a fresh checkout".
2. For Layer-2 (real API): a Cyclr Partner sandbox account with:
   - Partner-level OAuth client (client_id + client_secret)
   - At least one test Account inside that Partner with its API-ID
   - Optionally a separate Account-scoped OAuth client for cleaner scope isolation

---

## Layer 1 — Unit tests (mandatory for PR merge)

No credentials needed; tests use `mockserver`.

```bash
cd /Users/stefanbaxter/Development/saas-connectors
go test ./providers/cyclrpartner/... -count=1
go test ./providers/cyclraccount/... -count=1
make lint
```

Expected: green on both. This is the gate for every capability PR (1 through 9 in plan.md).

---

## Layer 2 — Real-API integration

Credentials live in `~/.ampersand-creds/`.

### `cyclrPartner` creds file

Path: `~/.ampersand-creds/cyclrPartner.json`

```json
{
  "provider": "cyclrPartner",
  "clientId": "YOUR_PARTNER_CLIENT_ID",
  "clientSecret": "YOUR_PARTNER_CLIENT_SECRET",
  "scopes": "",
  "metadata": {
    "apiDomain": "api.eu.cyclr.com"
  }
}
```

### `cyclrAccount` creds file

Path: `~/.ampersand-creds/cyclrAccount.json`

```json
{
  "provider": "cyclrAccount",
  "clientId": "YOUR_ACCOUNT_CLIENT_ID",
  "clientSecret": "YOUR_ACCOUNT_CLIENT_SECRET",
  "scopes": "account:00000000-0000-0000-0000-000000000000",
  "metadata": {
    "apiDomain": "api.eu.cyclr.com",
    "accountApiId": "00000000-0000-0000-0000-000000000000"
  }
}
```

Replace `00000000-0000-0000-0000-000000000000` with the target Account's API-ID.

### Run each test entrypoint

```bash
# Partner scope
go run ./test/cyclrPartner/metadata
go run ./test/cyclrPartner/read
go run ./test/cyclrPartner/write      # Creates a test Account, updates, suspends, resumes
go run ./test/cyclrPartner/delete     # Deletes the test Account created above

# Account scope
go run ./test/cyclrAccount/metadata
go run ./test/cyclrAccount/read
go run ./test/cyclrAccount/write      # Installs template, activates, deactivates
go run ./test/cyclrAccount/delete     # Deletes the installed Cycle
```

Each entrypoint is idempotent-ish: `write` cleans up after itself where possible, but failures may leave detritus in the Partner sandbox. Tolerable for a developer sandbox, not for shared environments.

---

## Layers 3–6 — Downstream gateway testing

Owned by `fraios/apps/saas-gateway`. See `DOWNSTREAM.md` for the full matrix. Summary:

- **Layer 3**: gateway unit tests after `go get github.com/amp-labs/connectors@latest` in the gateway repo.
- **Layer 4**: gateway local e2e with real cxs2 credentials and a real Cyclr sandbox.
- **Layer 5**: gateway e2e test suite (`internal/integration/`) — add a Cyclr case alongside existing ones.
- **Layer 6**: dev-cluster deploy with ArgoCD — watch `grafana.fraios.dev` for Cyclr request-rate and error metrics.

---

## Smoke test flow (end-to-end, Layer 4-adjacent)

A minimal "did it work?" flow you can run by hand once the connector is merged and the gateway bumped:

1. **Create a white-label Account** via `cyclrPartner`:
   ```
   POST /v1/write
   {
     "solution_link_id": "<partner-conn>",
     "object_name": "accounts",
     "record_data": {
       "Name": "Smoke-Test Account",
       "Timezone": "UTC"
     }
   }
   ```
   Capture the returned `Id`.

2. **Register a new gateway connection** for this Account — add `~/.ampersand-creds/cyclrAccount.json` with the new `accountApiId`, or, in the gateway flow, create a new `solution_link_id` bound to `cyclrAccount` with the new Account ID.

3. **List Cycles** in the new Account — should be empty:
   ```
   POST /v1/read
   {
     "solution_link_id": "<account-conn>",
     "object_name": "cycles"
   }
   ```

4. **Install a Cycle from a template**:
   ```
   POST /v1/write
   {
     "solution_link_id": "<account-conn>",
     "object_name": "cycles",
     "record_data": {
       "TemplateId": "<known-template-uuid>"
     }
   }
   ```

5. **Activate the Cycle**:
   ```
   POST /v1/write
   {
     "solution_link_id": "<account-conn>",
     "object_name": "cycles:activate",
     "record_id": "<cycle-id>",
     "record_data": {
       "Interval": 60,
       "RunOnce": false
     }
   }
   ```

6. **Deactivate and delete** — cleanup:
   ```
   POST /v1/write { "object_name": "cycles:deactivate", "record_id": "<cycle-id>" }
   POST /v1/delete { "object_name": "cycles", "record_id": "<cycle-id>" }
   ```

7. **Suspend then delete the Account** via `cyclrPartner`:
   ```
   POST /v1/write { "object_name": "accounts:suspend", "record_id": "<account-id>" }
   POST /v1/delete { "object_name": "accounts", "record_id": "<account-id>" }
   ```

If all seven steps return 2xx through the gateway, the feature is functionally verified end-to-end.

---

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| 401 from token endpoint on every call | `apiDomain` region doesn't match where the client was created. Cyclr isolates OAuth clients per region. |
| 403 on Account-scoped calls | Using Partner-scope credentials against `cyclrAccount` paths, or `scope=account:...` param missing from token request. |
| 429 wedging tests | Expected under heavy parallel runs. Retry budget (FR-061) kicks in automatically; re-run. |
| `X-Cyclr-Account` missing from requests (observed in test traces) | Middleware transport not wired in `constructor()`. Check `cyclraccount/connector.go`. |
| Schema metadata returns empty for `cycles` | `schemas.json` not embedded or `go:embed` directive missing. Check `metadata.go`. |
