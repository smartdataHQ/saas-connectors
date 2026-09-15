package asana

import (
	"net/http"
	"testing"

	"github.com/amp-labs/connectors"
	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockserver"
	"github.com/amp-labs/connectors/test/utils/testconn"
)

func TestReadWorkspacePagination(t *testing.T) {
	t.Parallel()

	const record = `{"gid":"123","name":"Example"}`
	const nextURL = "https://app.asana.com/api/1.0/workspaces?offset=opaque-token"

	for _, test := range []struct {
		name string
		body string
		next string
	}{
		{name: "null terminal page", body: `{"data":[` + record + `],"next_page":null}`},
		{name: "absent terminal page", body: `{"data":[` + record + `]}`},
		{name: "continuation", body: `{"data":[` + record + `],"next_page":{"uri":"` + nextURL + `"}}`, next: nextURL},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			row := map[string]any{"gid": "123", "name": "Example"}
			testCase := testconn.TestCaseRead{
				Input:  common.ReadParams{ObjectName: "workspaces", Fields: connectors.Fields("gid", "name")},
				Server: mockserver.Fixed{Setup: mockserver.ContentJSON(), Always: mockserver.Response(http.StatusOK, []byte(test.body))}.Server(),
				Expected: &common.ReadResult{
					Rows:     1,
					Data:     []common.ReadResultRow{{Fields: row, Raw: row}},
					NextPage: common.NextPageToken(test.next),
					Done:     test.next == "",
				},
			}
			testCase.Run(t, func() (testconn.TestableReader, error) {
				return constructTestConnector(testCase.Server.URL)
			})
		})
	}
}

func TestReadEmptyWorkspacePage(t *testing.T) {
	t.Parallel()

	testCase := testconn.TestCaseRead{
		Input:    common.ReadParams{ObjectName: "workspaces", Fields: connectors.Fields("gid")},
		Server:   mockserver.Fixed{Setup: mockserver.ContentJSON(), Always: mockserver.Response(http.StatusOK, []byte(`{"data":[],"next_page":null}`))}.Server(),
		Expected: &common.ReadResult{Data: []common.ReadResultRow{}, Done: true},
	}
	testCase.Run(t, func() (testconn.TestableReader, error) {
		return constructTestConnector(testCase.Server.URL)
	})
}

func TestReadMalformedWorkspacePagination(t *testing.T) {
	t.Parallel()

	testCase := testconn.TestCaseRead{
		Input:        common.ReadParams{ObjectName: "workspaces", Fields: connectors.Fields("gid")},
		Server:       mockserver.Fixed{Setup: mockserver.ContentJSON(), Always: mockserver.Response(http.StatusOK, []byte(`{"data":[{"gid":"123"}],"next_page":[]}`))}.Server(),
		ExpectedErrs: []error{jsonquery.ErrNotObject},
	}
	testCase.Run(t, func() (testconn.TestableReader, error) {
		return constructTestConnector(testCase.Server.URL)
	})
}
