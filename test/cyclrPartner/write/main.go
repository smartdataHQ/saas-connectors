package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/amp-labs/connectors/common"
	testharness "github.com/amp-labs/connectors/test/cyclrPartner"
	"github.com/amp-labs/connectors/test/utils"
)

func main() {
	ctx := context.Background()
	conn := testharness.GetCyclrPartnerConnector(ctx)

	name := fmt.Sprintf("SpecKit-US1-%d", time.Now().Unix())

	create, err := conn.Write(ctx, common.WriteParams{
		ObjectName: "accounts",
		RecordData: map[string]any{
			"Name":     name,
			"Timezone": "UTC",
		},
	})
	if err != nil {
		utils.Fail("create account failed", "error", err)
	}

	printResult("create", create)

	update, err := conn.Write(ctx, common.WriteParams{
		ObjectName: "accounts",
		RecordId:   create.RecordId,
		RecordData: map[string]any{
			"Description": "Updated by SpecKit US1 layer-2 harness",
		},
	})
	if err != nil {
		utils.Fail("update account failed", "error", err)
	}

	printResult("update", update)

	// NOTE: accounts:suspend / accounts:resume intentionally omitted. Layer-2
	// verification against api.cyclr.com returned 404 at every probed variant
	// — those endpoints don't appear to exist on the public Partner API.
	// See providers/cyclrpartner/objects.go and tasks.md T080.

	fmt.Println("Created AccountId:", create.RecordId)
}

func printResult(label string, result *common.WriteResult) {
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		utils.Fail("marshal result", "error", err)
	}

	fmt.Printf("=== %s ===\n%s\n", label, string(encoded))
}
