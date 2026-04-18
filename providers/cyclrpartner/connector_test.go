package cyclrpartner

import (
	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/test/utils/mockutils"
)

// constructTestConnector returns a Connector whose BaseURL has been rewritten
// to target a mockserver instance. Shared by read_test, write_test, delete_test.
func constructTestConnector(serverURL string) (*Connector, error) {
	conn, err := NewConnector(common.ConnectorParams{
		Module:              common.ModuleRoot,
		AuthenticatedClient: mockutils.NewClient(),
		Metadata: map[string]string{
			"apiDomain": "api.cyclr.com",
		},
	})
	if err != nil {
		return nil, err
	}

	conn.SetBaseURL(mockutils.ReplaceURLOrigin(conn.HTTPClient().Base, serverURL))

	return conn, nil
}
