package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/amp-labs/connectors/common"
	testharness "github.com/amp-labs/connectors/test/cyclrPartner"
	"github.com/amp-labs/connectors/test/utils"
)

func main() {
	var accountID string

	flag.StringVar(&accountID, "id", "", "Account Id to delete")
	flag.Parse()

	if accountID == "" {
		utils.Fail("--id is required (pass the AccountId created by test/cyclrPartner/write)")
	}

	ctx := context.Background()
	conn := testharness.GetCyclrPartnerConnector(ctx)

	result, err := conn.Delete(ctx, common.DeleteParams{
		ObjectName: "accounts",
		RecordId:   accountID,
	})
	if err != nil {
		utils.Fail("delete account failed", "error", err)
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		utils.Fail("marshal result", "error", err)
	}

	fmt.Println(string(encoded))
}
