package cyclraccount

import (
	"net/http"
	"testing"

	"github.com/amp-labs/connectors"
	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockcond"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockserver"
	"github.com/amp-labs/connectors/test/utils/testroutines"
)

func TestRead(t *testing.T) {
	t.Parallel()

	cyclesPage := []byte(`[
		{"Id":"c0000001-0000-0000-0000-000000000001","Name":"Sample Cycle","Status":"Paused","TemplateId":"t0000001-0000-0000-0000-000000000001","ErrorCount":0}
	]`)

	tests := []testroutines.Read{
		{
			Name: "List templates in Account scope routes to /v1.0/templates",
			Input: common.ReadParams{
				ObjectName: "templates",
				Fields:     connectors.Fields("Id", "Name"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1.0/templates"),
					mockcond.Header(http.Header{
						headerAccountApiId: []string{testAccountApiId},
					}),
				},
				Then: mockserver.Response(http.StatusOK, []byte(`[
					{"Id":"t1","Name":"Template One"}
				]`)),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Fields: map[string]any{"id": "t1", "name": "Template One"},
					Raw:    map[string]any{"Name": "Template One"},
				}},
				Done: true,
			},
		},
		{
			Name: "Parent-scoped list: cycles/{cycleId}/steps routes to /v1.0/cycles/{id}/steps",
			Input: common.ReadParams{
				ObjectName: "cycles/c0000001-0000-0000-0000-000000000001/steps",
				Fields:     connectors.Fields("Id", "Name", "StepType"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1.0/cycles/c0000001-0000-0000-0000-000000000001/steps"),
				},
				Then: mockserver.Response(http.StatusOK, []byte(`[
					{"Id":"s1","Name":"Step One","StepType":"Action","CycleId":"c0000001-0000-0000-0000-000000000001"}
				]`)),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Fields: map[string]any{
						"id":       "s1",
						"name":     "Step One",
						"steptype": "Action",
					},
					Raw: map[string]any{"Name": "Step One"},
				}},
				Done: true,
			},
		},
		{
			Name: "Parent-scoped list: steps/{stepId}/parameters strips credential-like values",
			Input: common.ReadParams{
				ObjectName: "steps/s1/parameters",
				Fields:     connectors.Fields("Id", "Name", "Value", "ApiKey"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1.0/steps/s1/parameters"),
				},
				Then: mockserver.Response(http.StatusOK, []byte(`[
					{"Id":"p1","Name":"Token","Value":"abc","ApiKey":"secret-value"}
				]`)),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Fields: map[string]any{
						"id":     "p1",
						"name":   "Token",
						"value":  "abc",
						"apikey": "", // stripped by credential heuristic
					},
					// Raw preserves the original Cyclr response verbatim,
					// including the secret-shaped field (FR-051).
					Raw: map[string]any{"ApiKey": "secret-value"},
				}},
				Done: true,
			},
		},
		{
			Name: "accountConnectors list routes to /v1.0/account/connectors and strips AuthValue",
			Input: common.ReadParams{
				ObjectName: objectNameAccountConnectors,
				Fields:     connectors.Fields("Id", "Name", "AuthenticationState", "AuthValue"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1.0/account/connectors"),
				},
				Then: mockserver.Response(http.StatusOK, []byte(`[
					{"Id":"ac1","Name":"SFDC Install","AuthenticationState":"Authenticated","AuthValue":"should-not-appear"}
				]`)),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Fields: map[string]any{
						"id":                  "ac1",
						"name":                "SFDC Install",
						"authenticationstate": "Authenticated",
						"authvalue":           "", // stripped
					},
					Raw: map[string]any{"Name": "SFDC Install"},
				}},
				Done: true,
			},
		},
		{
			Name: "List cycles attaches X-Cyclr-Account header and returns rows",
			Input: common.ReadParams{
				ObjectName: objectNameCycles,
				Fields:     connectors.Fields("Id", "Status"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1.0/cycles"),
					mockcond.Header(http.Header{
						headerAccountApiId: []string{testAccountApiId},
					}),
				},
				Then: mockserver.ResponseChainedFuncs(
					mockserver.Header("Total-Pages", "1"),
					mockserver.Response(http.StatusOK, cyclesPage),
				),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Fields: map[string]any{
						"id":     "c0000001-0000-0000-0000-000000000001",
						"status": "Paused",
					},
					Raw: map[string]any{
						"Name": "Sample Cycle",
					},
				}},
				NextPage: "",
				Done:     true,
			},
			ExpectedErrs: nil,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			tt.Run(t, func() (connectors.ReadConnector, error) {
				return constructTestConnector(tt.Server.URL)
			})
		})
	}
}
