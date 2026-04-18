package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amp-labs/connectors"
	"github.com/amp-labs/connectors/common"
	testharness "github.com/amp-labs/connectors/test/cyclrAccount"
	"github.com/amp-labs/connectors/test/utils"
)

func main() {
	ctx := context.Background()
	conn := testharness.GetCyclrAccountConnector(ctx)

	for _, obj := range []struct {
		name   string
		fields []string
	}{
		{"cycles", []string{"Id", "Name", "Status", "TemplateId"}},
		{"templates", []string{"Id", "Name", "Category"}},
		{"connectors", []string{"Id", "Name", "AuthenticationType"}},
		{"accountConnectors", []string{"Id", "Name", "AuthenticationState"}},
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
