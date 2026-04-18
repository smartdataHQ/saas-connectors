package cyclraccount

import (
	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/internal/datautils"
)

const (
	objectNameCycles           = "cycles"
	objectNameCyclesActivate   = "cycles:activate"
	objectNameCyclesDeactivate = "cycles:deactivate"

	objectNameCycleSteps              = "cycleSteps"
	objectNameCycleStepsPrerequisites = "cycleSteps:prerequisites"

	objectNameStepParameters    = "stepParameters"
	objectNameStepFieldMappings = "stepFieldMappings"

	objectNameAccountConnectors = "accountConnectors"

	// Parent-scoped glob patterns. These register ReadSupport on globs like
	// `cycles/{cycleId}/steps` so callers can surface the cycleId as part of
	// the object name. common.ReadParams has no auxiliary-id slot, so this
	// is how we express "children of X".
	endpointPatternCyclesSteps             = "cycles/*/steps"
	endpointPatternStepsParameters         = "steps/*/parameters"
	endpointPatternStepsFieldmappings      = "steps/*/fieldmappings"
	endpointPatternStepsPrerequisites      = "steps/*/prerequisites"
	parentPathSegmentCycles                = "cycles"
	parentPathSegmentSteps                 = "steps"
	parentChildSegmentCycleSteps           = "steps"
	parentChildSegmentStepParameters       = "parameters"
	parentChildSegmentStepFieldMappings    = "fieldmappings"
	parentChildSegmentPrerequisites        = "prerequisites"
	accountConnectorsPath                  = "account/connectors"
	payloadKeyStepId                       = "StepId"
	payloadKeyConnectorId                  = "ConnectorId"
	payloadKeyAuthValue                    = "AuthValue"
	payloadKeyMappingType                  = "MappingType"
	payloadKeyValue                        = "Value"
	payloadKeySourceStepId                 = "SourceStepId"
	payloadKeySourceFieldName              = "SourceFieldName"
	payloadKeyVariableName                 = "VariableName"
	payloadKeyName                         = "Name"
	payloadKeyDescription                  = "Description"
)

//nolint:gochecknoglobals
var supportedObjectsByCreate = map[common.ModuleID]datautils.StringSet{
	common.ModuleRoot: datautils.NewSet(
		// Create maps to Cyclr's install-from-template flow:
		// POST /v1.0/templates/{TemplateId}/install.
		objectNameCycles,
		objectNameCyclesActivate,
		objectNameCyclesDeactivate,
		// Install a catalog Connector into the Account.
		objectNameAccountConnectors,
	),
}

//nolint:gochecknoglobals
var supportedObjectsByUpdate = map[common.ModuleID]datautils.StringSet{
	common.ModuleRoot: datautils.NewSet(
		// Step parameter / field-mapping updates carry the StepId in
		// RecordData; RecordId is the parameter / mapping Id.
		objectNameStepParameters,
		objectNameStepFieldMappings,
	),
}

//nolint:gochecknoglobals
var supportedObjectsByDelete = map[common.ModuleID]datautils.StringSet{
	common.ModuleRoot: datautils.NewSet(
		objectNameCycles,
	),
}
