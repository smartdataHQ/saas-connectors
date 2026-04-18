package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amp-labs/connectors"
	"github.com/amp-labs/connectors/common"
	testharness "github.com/amp-labs/connectors/test/cyclrPartner"
	"github.com/amp-labs/connectors/test/utils"
)

func main() {
	ctx := context.Background()
	conn := testharness.GetCyclrPartnerConnector(ctx)

	for _, obj := range []struct {
		name   string
		fields []string
	}{
		{"accounts", []string{"Id", "Name", "Timezone", "CreatedDate"}},
		{"templates", []string{"Id", "Name", "Category"}},
	} {
		result, err := conn.Read(ctx, common.ReadParams{
			ObjectName: obj.name,
			Fields:     connectors.Fields(obj.fields...),
		})
		if err != nil {
			utils.Fail("Read failed", "object", obj.name, "error", err)
		}

		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			utils.Fail("marshal result", "error", err)
		}

		fmt.Printf("=== %s ===\n%s\n", obj.name, string(encoded))
	}
}
