package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/amp-labs/connectors/common"
	testharness "github.com/amp-labs/connectors/test/cyclrAccount"
	"github.com/amp-labs/connectors/test/utils"
)

func main() {
	var cycleID string

	flag.StringVar(&cycleID, "id", "", "CycleId to delete")
	flag.Parse()

	if cycleID == "" {
		utils.Fail("--id is required (pass the CycleId created by test/cyclrAccount/write)")
	}

	ctx := context.Background()
	conn := testharness.GetCyclrAccountConnector(ctx)

	result, err := conn.Delete(ctx, common.DeleteParams{
		ObjectName: "cycles",
		RecordId:   cycleID,
	})
	if err != nil {
		utils.Fail("delete cycle failed", "error", err)
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		utils.Fail("marshal result", "error", err)
	}

	fmt.Println(string(encoded))
}
