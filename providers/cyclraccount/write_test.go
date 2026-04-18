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

func TestWrite(t *testing.T) { //nolint:funlen
	t.Parallel()

	installedCycle := []byte(`{
		"Id":"c0000001-0000-0000-0000-000000000001",
		"Name":"Installed Cycle",
		"TemplateId":"t0000001-0000-0000-0000-000000000001",
		"Status":"Paused",
		"ErrorCount":0,
		"WarningCount":0,
		"CreatedOnUtc":"2026-04-18T10:00:00Z"
	}`)

	tests := []testroutines.Write{
		{
			Name: "Install from template returns new Cycle Id",
			Input: common.WriteParams{
				ObjectName: objectNameCycles,
				RecordData: map[string]any{
					"TemplateId": "t0000001-0000-0000-0000-000000000001",
				},
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodPOST(),
					mockcond.Path("/v1.0/templates/t0000001-0000-0000-0000-000000000001/install"),
				},
				Then: mockserver.Response(http.StatusOK, installedCycle),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetWrite,
			Expected: &common.WriteResult{
				Success:  true,
				RecordId: "c0000001-0000-0000-0000-000000000001",
			},
		},
		{
			Name: "Activate with valid Interval succeeds",
			Input: common.WriteParams{
				ObjectName: objectNameCyclesActivate,
				RecordId:   "c0000001-0000-0000-0000-000000000001",
				RecordData: map[string]any{
					"Interval": 60,
					"RunOnce":  false,
				},
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodPUT(),
					mockcond.Path("/v1.0/cycles/c0000001-0000-0000-0000-000000000001/activate"),
				},
				Then: mockserver.Response(http.StatusNoContent),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetWrite,
			Expected: &common.WriteResult{
				Success:  true,
				RecordId: "c0000001-0000-0000-0000-000000000001",
			},
		},
		{
			Name: "Activate with invalid Interval is rejected client-side",
			Input: common.WriteParams{
				ObjectName: objectNameCyclesActivate,
				RecordId:   "c0000001-0000-0000-0000-000000000001",
				RecordData: map[string]any{
					"Interval": 7, // not in the allowed set
				},
			},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrBadRequest},
		},
		{
			Name: "Update stepParameters forwards mapping-only body and strips StepId",
			Input: common.WriteParams{
				ObjectName: objectNameStepParameters,
				RecordId:   "p1",
				RecordData: map[string]any{
					"StepId":      "s1",
					"MappingType": "StaticValue",
					"Value":       "hello",
				},
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodPUT(),
					mockcond.Path("/v1.0/steps/s1/parameters/p1"),
					// Body MUST contain MappingType+Value; MUST NOT contain StepId.
					mockcond.Body(`{"MappingType":"StaticValue","Value":"hello"}`),
				},
				Then: mockserver.Response(http.StatusOK, []byte(`{"Id":"p1"}`)),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetWrite,
			Expected: &common.WriteResult{
				Success:  true,
				RecordId: "p1",
			},
		},
		{
			Name: "Update stepParameters rejects missing StepId",
			Input: common.WriteParams{
				ObjectName: objectNameStepParameters,
				RecordId:   "p1",
				RecordData: map[string]any{
					// StepId intentionally omitted.
					"MappingType": "StaticValue",
					"Value":       "x",
				},
			},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrBadRequest},
		},
		{
			Name: "Update stepFieldMappings routes to /fieldmappings/{id}",
			Input: common.WriteParams{
				ObjectName: objectNameStepFieldMappings,
				RecordId:   "f1",
				RecordData: map[string]any{
					"StepId":      "s1",
					"MappingType": "StepOutput",
					"SourceStepId":    "upstream",
					"SourceFieldName": "email",
				},
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodPUT(),
					mockcond.Path("/v1.0/steps/s1/fieldmappings/f1"),
				},
				Then: mockserver.Response(http.StatusOK, []byte(`{"Id":"f1"}`)),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetWrite,
			Expected: &common.WriteResult{
				Success:  true,
				RecordId: "f1",
			},
		},
		{
			Name: "Install accountConnector routes to /v1.0/connectors/{ConnectorId}/install",
			Input: common.WriteParams{
				ObjectName: objectNameAccountConnectors,
				RecordData: map[string]any{
					"ConnectorId": "catalog-connector-1",
					"Name":        "SFDC Install",
					"Description": "CI install",
					"AuthValue":   "secret-key",
				},
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodPOST(),
					mockcond.Path("/v1.0/connectors/catalog-connector-1/install"),
				},
				Then: mockserver.Response(http.StatusOK, []byte(`{"Id":"ac1"}`)),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetWrite,
			Expected: &common.WriteResult{
				Success:  true,
				RecordId: "ac1",
			},
		},
		{
			Name: "Install accountConnector without ConnectorId fails fast",
			Input: common.WriteParams{
				ObjectName: objectNameAccountConnectors,
				RecordData: map[string]any{
					"Name": "Missing ConnectorId",
				},
			},
			Server:       mockserver.Dummy(),
			ExpectedErrs: []error{common.ErrBadRequest},
		},
		{
			Name: "Deactivate with empty body",
			Input: common.WriteParams{
				ObjectName: objectNameCyclesDeactivate,
				RecordId:   "c0000001-0000-0000-0000-000000000001",
				RecordData: map[string]any{},
			},
			Server: mockserver.Conditional{
				Setup: mockserver.ContentJSON(),
				If: mockcond.And{
					mockcond.MethodPUT(),
					mockcond.Path("/v1.0/cycles/c0000001-0000-0000-0000-000000000001/deactivate"),
				},
				Then: mockserver.Response(http.StatusNoContent),
			}.Server(),
			Comparator: testroutines.ComparatorSubsetWrite,
			Expected: &common.WriteResult{
				Success:  true,
				RecordId: "c0000001-0000-0000-0000-000000000001",
			},
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
