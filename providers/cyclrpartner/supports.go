package cyclrpartner

import (
	"fmt"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/components"
	"github.com/amp-labs/connectors/internal/datautils"
)

// supportedOperations declares which object names the Partner scope accepts
// for each of Read / Write / Delete. Patterns follow gobwas/glob syntax;
// `{a,b,c}` means any of the listed literals. See:
// https://github.com/gobwas/glob
//
// accounts is the only typed object in MVP US1; templates / connectors come
// in Phase 5 (US3) via T064.
func supportedOperations() components.EndpointRegistryInput {
	// Reads exactly track the schema registry. Each object with a schema
	// entry has ReadSupport automatically.
	readSupport := schemas.ObjectNames().GetList(common.ModuleRoot)

	writeSupport := datautils.MergeUniqueLists(
		supportedObjectsByCreate,
		supportedObjectsByUpdate,
	).GetList(common.ModuleRoot)

	deleteSupport := supportedObjectsByDelete[common.ModuleRoot].List()

	return components.EndpointRegistryInput{
		common.ModuleRoot: {
			{
				Endpoint: fmt.Sprintf("{%s}", strings.Join(readSupport, ",")),
				Support:  components.ReadSupport,
			},
			{
				Endpoint: fmt.Sprintf("{%s}", strings.Join(writeSupport, ",")),
				Support:  components.WriteSupport,
			},
			{
				Endpoint: fmt.Sprintf("{%s}", strings.Join(deleteSupport, ",")),
				Support:  components.DeleteSupport,
			},
		},
	}
}
