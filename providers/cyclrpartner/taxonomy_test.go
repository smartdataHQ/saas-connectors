package cyclrpartner

import (
	"testing"

	"github.com/amp-labs/connectors/common"
)

// TestObjectNameTaxonomy exercises FR-049: every name registered in the
// cyclrPartner supports.go must fall inside the published taxonomy. The
// downstream MCP gateway groups tools by prefix / suffix on these names;
// drifting away from the taxonomy without an explicit spec amendment is a
// breaking change for the gateway's tool ID scheme.
//
// For cyclrPartner the allowed groups are:
//   - Account lifecycle: accounts, accounts:suspend, accounts:resume
//   - Catalog:           templates, connectors
func TestObjectNameTaxonomy(t *testing.T) {
	t.Parallel()

	allowed := map[string]struct{}{
		// Account lifecycle
		objectNameAccounts:        {},
		objectNameAccountsSuspend: {},
		objectNameAccountsResume: {},
		// Catalog (Partner-scope)
		"templates": {},
	}

	registry := supportedOperations()[common.ModuleRoot]
	seen := make(map[string]struct{})

	// supports.go composes `{a,b,c}` glob expressions. Walk the registered
	// schema object names (used for ReadSupport) and the create/update/delete
	// sets (used for Write/Delete) to assemble the full registered set.
	for _, name := range schemas.ObjectNames().GetList(common.ModuleRoot) {
		seen[name] = struct{}{}
	}

	for name := range supportedObjectsByCreate[common.ModuleRoot] {
		seen[name] = struct{}{}
	}

	for name := range supportedObjectsByUpdate[common.ModuleRoot] {
		seen[name] = struct{}{}
	}

	for name := range supportedObjectsByDelete[common.ModuleRoot] {
		seen[name] = struct{}{}
	}

	for name := range seen {
		if _, ok := allowed[name]; !ok {
			t.Errorf("object name %q is outside the FR-049 taxonomy for cyclrPartner", name)
		}
	}

	// Ensure the registry is non-empty (guards against the test silently
	// passing if the package refactors supports.go into a no-op).
	if len(registry) == 0 {
		t.Errorf("no endpoints registered — supports.go regressed")
	}
}
