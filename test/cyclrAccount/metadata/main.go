package main

import (
	"context"
	"encoding/json"
	"fmt"

	testharness "github.com/amp-labs/connectors/test/cyclrAccount"
	"github.com/amp-labs/connectors/test/utils"
)

func main() {
	ctx := context.Background()
	conn := testharness.GetCyclrAccountConnector(ctx)

	result, err := conn.ListObjectMetadata(ctx, []string{"cycles"})
	if err != nil {
		utils.Fail("ListObjectMetadata failed", "error", err)
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		utils.Fail("marshal result", "error", err)
	}

	fmt.Println(string(encoded))
}
