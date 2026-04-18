package cyclraccount

import (
	"fmt"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/components"
	"github.com/amp-labs/connectors/internal/datautils"
)

// supportedOperations declares which object names the Account scope accepts
// for each of Read / Write / Delete. Patterns follow gobwas/glob syntax;
// `{a,b,c}` means any of the listed literals, `*` is a wildcard path segment.
// See: https://github.com/gobwas/glob.
func supportedOperations() components.EndpointRegistryInput {
	// ReadSupport comes from two sources:
	//   1. Every schema-registered object (literal match).
	//   2. Parent-scoped glob patterns — how we surface "list children of X"
	//      in a library whose ReadParams carries no auxiliary id.
	readSupport := schemas.ObjectNames().GetList(common.ModuleRoot)

	writeSupport := datautils.MergeUniqueLists(
		supportedObjectsByCreate,
		supportedObjectsByUpdate,
	).GetList(common.ModuleRoot)

	deleteSupport := supportedObjectsByDelete[common.ModuleRoot].List()

	return components.EndpointRegistryInput{
		common.ModuleRoot: {
			// Literal reads from the schema registry.
			{
				Endpoint: fmt.Sprintf("{%s}", strings.Join(readSupport, ",")),
				Support:  components.ReadSupport,
			},
			// Parent-scoped list globs.
			{Endpoint: endpointPatternCyclesSteps, Support: components.ReadSupport},
			{Endpoint: endpointPatternStepsParameters, Support: components.ReadSupport},
			{Endpoint: endpointPatternStepsFieldmappings, Support: components.ReadSupport},
			{Endpoint: endpointPatternStepsPrerequisites, Support: components.ReadSupport},
			// Writes.
			{
				Endpoint: fmt.Sprintf("{%s}", strings.Join(writeSupport, ",")),
				Support:  components.WriteSupport,
			},
			// Deletes.
			{
				Endpoint: fmt.Sprintf("{%s}", strings.Join(deleteSupport, ",")),
				Support:  components.DeleteSupport,
			},
		},
	}
}
