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
	var templateID string

	flag.StringVar(&templateID, "template", "", "TemplateId to install into this Account")
	flag.Parse()

	if templateID == "" {
		utils.Fail("--template is required (TemplateId to install into this Account)")
	}

	ctx := context.Background()
	conn := testharness.GetCyclrAccountConnector(ctx)

	install, err := conn.Write(ctx, common.WriteParams{
		ObjectName: "cycles",
		RecordData: map[string]any{
			"TemplateId": templateID,
		},
	})
	if err != nil {
		utils.Fail("install-from-template failed", "error", err)
	}

	printResult("install", install)

	activate, err := conn.Write(ctx, common.WriteParams{
		ObjectName: "cycles:activate",
		RecordId:   install.RecordId,
		RecordData: map[string]any{
			"Interval": 60,
			"RunOnce":  false,
		},
	})
	if err != nil {
		utils.Fail("activate failed", "error", err)
	}

	printResult("activate", activate)

	deactivate, err := conn.Write(ctx, common.WriteParams{
		ObjectName: "cycles:deactivate",
		RecordId:   install.RecordId,
		RecordData: map[string]any{},
	})
	if err != nil {
		utils.Fail("deactivate failed", "error", err)
	}

	printResult("deactivate", deactivate)

	fmt.Println("Installed CycleId:", install.RecordId)
}

func printResult(label string, result *common.WriteResult) {
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		utils.Fail("marshal result", "error", err)
	}

	fmt.Printf("=== %s ===\n%s\n", label, string(encoded))
}
