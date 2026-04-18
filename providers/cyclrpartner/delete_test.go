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

func TestDelete(t *testing.T) {
	t.Parallel()

	refusalBody := []byte(`{"Message":"Cannot delete Account with active Cycles."}`)

	tests := []testroutines.Delete{
		{
			Name: "Delete account succeeds on 204",
			Input: common.DeleteParams{
				ObjectName: objectNameAccounts,
				RecordId:   "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodDELETE(),
					mockcond.Path("/v1.0/accounts/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				},
				Then: mockserver.Response(http.StatusNoContent),
			}.Server(),
			Expected: &common.DeleteResult{Success: true},
		},
		{
			Name: "Delete refusal (422) surfaces typed error",
			Input: common.DeleteParams{
				ObjectName: objectNameAccounts,
				RecordId:   "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnprocessableEntity, refusalBody),
			}.Server(),
			ExpectedErrs: []error{common.ErrBadRequest},
		},
		{
			Name: "Delete missing RecordId fails fast",
			Input: common.DeleteParams{
				ObjectName: objectNameAccounts,
				RecordId:   "",
			},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrMissingRecordID},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			tt.Run(t, func() (connectors.DeleteConnector, error) {
				return constructTestConnector(tt.Server.URL)
			})
		})
	}
}
