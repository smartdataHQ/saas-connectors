package cyclrpartner

import (
	"net/http"
	"testing"

	"github.com/amp-labs/connectors"
	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockcond"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockserver"
	"github.com/amp-labs/connectors/test/utils/testroutines"
)

func TestRead(t *testing.T) { //nolint:funlen
	t.Parallel()

	firstPageResponse := []byte(`[
		{"Id":"11111111-1111-1111-1111-111111111111","Name":"Alpha Account","Enabled":true,"CreatedOnUtc":"2026-04-01T00:00:00Z"},
		{"Id":"22222222-2222-2222-2222-222222222222","Name":"Bravo Account","Enabled":true,"CreatedOnUtc":"2026-04-02T00:00:00Z"}
	]`)

	lastPageResponse := []byte(`[
		{"Id":"33333333-3333-3333-3333-333333333333","Name":"Charlie Account","Enabled":false,"CreatedOnUtc":"2026-04-03T00:00:00Z"}
	]`)

	emptyPageResponse := []byte(`[]`)

	errorResponse := []byte(`{"Message":"The resource you are looking for could not be found."}`)

	tests := []testroutines.Read{
		{
			Name: "List accounts first page has next-page token",
			Input: common.ReadParams{
				ObjectName: objectNameAccounts,
				Fields:     connectors.Fields("Id", "Name", "Enabled"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1.0/accounts"),
					mockcond.QueryParam("page", "1"),
					mockcond.QueryParam("per_page", "50"),
				},
				Then: mockserver.ResponseChainedFuncs(
					mockserver.Header("Total-Pages", "2"),
					mockserver.Response(http.StatusOK, firstPageResponse),
				),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Fields: map[string]any{
						"id":   "11111111-1111-1111-1111-111111111111",
						"name": "Alpha Account",
					},
					Raw: map[string]any{
						"Name": "Alpha Account",
					},
				}, {
					Fields: map[string]any{
						"id":   "22222222-2222-2222-2222-222222222222",
						"name": "Bravo Account",
					},
					Raw: map[string]any{
						"Name": "Bravo Account",
					},
				}},
				NextPage: "2",
				Done:     false,
			},
			ExpectedErrs: nil,
		},
		{
			Name: "List accounts last page has empty next-page",
			Input: common.ReadParams{
				ObjectName: objectNameAccounts,
				Fields:     connectors.Fields("Id"),
				NextPage:   "2",
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.QueryParam("page", "2"),
				},
				Then: mockserver.ResponseChainedFuncs(
					mockserver.Header("Total-Pages", "2"),
					mockserver.Response(http.StatusOK, lastPageResponse),
				),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 1,
				Data: []common.ReadResultRow{{
					Fields: map[string]any{
						"id": "33333333-3333-3333-3333-333333333333",
					},
					Raw: map[string]any{
						"Name": "Charlie Account",
					},
				}},
				NextPage: "",
				Done:     true,
			},
			ExpectedErrs: nil,
		},
		{
			Name: "Empty list returns Done with zero rows",
			Input: common.ReadParams{
				ObjectName: objectNameAccounts,
				Fields:     connectors.Fields("Id"),
			},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusOK, emptyPageResponse),
			}.Server(),
			Comparator: testroutines.ComparatorPagination,
			Expected: &common.ReadResult{
				Rows:     0,
				NextPage: "",
				Done:     true,
			},
			ExpectedErrs: nil,
		},
		{
			Name: "404 surfaces typed not-found error with Cyclr message",
			Input: common.ReadParams{
				ObjectName: objectNameAccounts,
				Fields:     connectors.Fields("Id"),
			},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusNotFound, errorResponse),
			}.Server(),
			ExpectedErrs: []error{common.ErrNotFound},
		},
		{
			Name: "List templates routes to /v1.0/templates",
			Input: common.ReadParams{
				ObjectName: "templates",
				Fields:     connectors.Fields("Id", "Name"),
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodGET(),
					mockcond.Path("/v1.0/templates"),
				},
				Then: mockserver.Response(http.StatusOK, []byte(`[
					{"Id":"t1","Name":"Template One"},
					{"Id":"t2","Name":"Template Two"}
				]`)),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetRead,
			Expected: &common.ReadResult{
				Rows: 2,
				Data: []common.ReadResultRow{{
					Fields: map[string]any{"id": "t1", "name": "Template One"},
					Raw:    map[string]any{"Name": "Template One"},
				}, {
					Fields: map[string]any{"id": "t2", "name": "Template Two"},
					Raw:    map[string]any{"Name": "Template Two"},
				}},
				Done: true,
			},
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
