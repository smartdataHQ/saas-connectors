package cyclrpartner

import (
	"net/http"
	"testing"

	"github.com/amp-labs/connectors"
	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockcond"
	"github.com/amp-labs/connectors/test/utils/mockutils/mockserver"
	"github.com/amp-labs/connectors/test/utils/testroutines"
	"github.com/amp-labs/connectors/test/utils/testutils"
)

func TestWrite(t *testing.T) { //nolint:funlen
	t.Parallel()

	createdAccount := []byte(`{
		"Id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"Name":"New Account",
		"Timezone":"UTC",
		"Enabled":true,
		"CreatedOnUtc":"2026-04-18T10:00:00Z"
	}`)

	updatedAccount := []byte(`{
		"Id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"Name":"Renamed Account",
		"Description":"Updated"
	}`)

	validationError := []byte(`{
		"Message":"The request is invalid.",
		"ModelState":{"Timezone":["Timezone is required"]}
	}`)

	authError := []byte(`{"Message":"Authorization has been denied for this request."}`)

	tests := []testroutines.Write{
		{
			Name: "Create account returns assigned Id",
			Input: common.WriteParams{
				ObjectName: objectNameAccounts,
				RecordData: map[string]any{
					"Name":     "New Account",
					"Timezone": "UTC",
				},
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodPOST(),
					mockcond.Path("/v1.0/accounts"),
				},
				Then: mockserver.Response(http.StatusOK, createdAccount),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetWrite,
			Expected: &common.WriteResult{
				Success:  true,
				RecordId: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			},
		},
		{
			Name: "Update account by RecordId returns Id",
			Input: common.WriteParams{
				ObjectName: objectNameAccounts,
				RecordId:   "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				RecordData: map[string]any{
					"Description": "Updated",
				},
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodPUT(),
					mockcond.Path("/v1.0/accounts/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				},
				Then: mockserver.Response(http.StatusOK, updatedAccount),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetWrite,
			Expected: &common.WriteResult{
				Success:  true,
				RecordId: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			},
		},
		{
			Name: "Suspend returns typed unsupported error (Layer-2 finding: endpoint does not exist)",
			Input: common.WriteParams{
				ObjectName: objectNameAccountsSuspend,
				RecordId:   "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				RecordData: map[string]any{},
			},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrOperationNotSupportedForObject},
		},
		{
			Name: "Validation error (422) surfaces ModelState detail",
			Input: common.WriteParams{
				ObjectName: objectNameAccounts,
				RecordData: map[string]any{
					"Name": "Missing Timezone",
				},
			},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnprocessableEntity, validationError),
			}.Server(),
			ExpectedErrs: []error{
				common.ErrBadRequest,
				testutils.StringError("Timezone: Timezone is required"),
			},
		},
		{
			Name: "Auth error (401) surfaces typed access-token error",
			Input: common.WriteParams{
				ObjectName: objectNameAccounts,
				RecordData: map[string]any{"Name": "Any"},
			},
			Server: mockserver.Fixed{
				Setup:  mockserver.ContentJSON(),
				Always: mockserver.Response(http.StatusUnauthorized, authError),
			}.Server(),
			ExpectedErrs: []error{common.ErrAccessToken},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			tt.Run(t, func() (connectors.WriteConnector, error) {
				return constructTestConnector(tt.Server.URL)
			})
		})
	}
}
