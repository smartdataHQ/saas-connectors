package cyclrpartner

import (
	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/datautils"
)

// Object names for the Partner scope. Cyclr preserves PascalCase in payloads
// but uses lowercase path segments — the object names here match the path
// segment convention and are what callers supply in ReadParams.ObjectName.
//
// NOTE on suspend / resume: the spec (FR-012, FR-013) and contract assumed
// Cyclr exposes `POST /v1.0/accounts/{id}/{suspend,resume}` Partner endpoints.
// Layer-2 probing on api.cyclr.com returned 404 at every variant tried
// (POST/PUT, Partner-scope, Account-scope with X-Cyclr-Account, alternate
// capitalisation). These routes do not appear to exist in the current public
// Partner API. Names reserved for documentation / future re-enablement; NOT
// registered as typed writes until Cyclr confirms a working path.
const (
	objectNameAccounts        = "accounts"
	objectNameAccountsSuspend = "accounts:suspend"
	objectNameAccountsResume  = "accounts:resume"
)

//nolint:gochecknoglobals
var supportedObjectsByCreate = map[common.ModuleID]datautils.StringSet{
	common.ModuleRoot: datautils.NewSet(
		objectNameAccounts,
	),
}

//nolint:gochecknoglobals
var supportedObjectsByUpdate = map[common.ModuleID]datautils.StringSet{
	common.ModuleRoot: datautils.NewSet(
		objectNameAccounts,
	),
}

//nolint:gochecknoglobals
var supportedObjectsByDelete = map[common.ModuleID]datautils.StringSet{
	common.ModuleRoot: datautils.NewSet(
		objectNameAccounts,
	),
}
