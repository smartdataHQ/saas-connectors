package cyclraccount

import (
	"testing"

	"github.com/amp-labs/connectors/common"
)

// TestObjectNameTaxonomy — see cyclrpartner/taxonomy_test.go for rationale.
//
// For cyclrAccount the allowed groups from FR-049 are:
//   - Cycle control:       cycles, cycles:activate, cycles:deactivate
//   - Cycle diagnostics:   cycleSteps, cycleSteps:prerequisites (plus glob
//                          cycles/*/steps — not in the schema map, registered
//                          directly in supports.go)
//   - Step configuration:  stepParameters, stepFieldMappings (plus globs
//                          steps/*/parameters, steps/*/fieldmappings,
//                          steps/*/prerequisites — same note)
//   - Connector setup:     accountConnectors
//   - Catalog:             templates (read-only view)
func TestObjectNameTaxonomy(t *testing.T) {
	t.Parallel()

	allowed := map[string]struct{}{
		// Cycle control
		objectNameCycles:           {},
		objectNameCyclesActivate:   {},
		objectNameCyclesDeactivate: {},
		// Cycle diagnostics
		objectNameCycleSteps:              {},
		objectNameCycleStepsPrerequisites: {},
		// Step configuration
		objectNameStepParameters:    {},
		objectNameStepFieldMappings: {},
		// Connector setup
		objectNameAccountConnectors: {},
		// Catalog
		"templates":  {},
		"connectors": {},
	}

	seen := make(map[string]struct{})

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
			t.Errorf("object name %q is outside the FR-049 taxonomy for cyclrAccount", name)
		}
	}
}
