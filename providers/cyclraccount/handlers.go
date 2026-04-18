package cyclraccount

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/urlbuilder"
	"github.com/amp-labs/connectors/internal/jsonquery"
	"github.com/spyzhov/ajson"
)

const (
	defaultPerPage  = "50"
	queryParamPage  = "page"
	queryParamSize  = "per_page"
	headerTotalPage = "Total-Pages"

	pathSegmentInstall    = "install"
	pathSegmentActivate   = "activate"
	pathSegmentDeactivate = "deactivate"

	payloadKeyTemplateId = "TemplateId"
	payloadKeyStartTime  = "StartTime"
	payloadKeyInterval   = "Interval"
	payloadKeyRunOnce    = "RunOnce"
)

// allowedCycleIntervals mirrors data-model.md §Cycle.Interval and is enforced
// client-side before the activate call goes out.
//
//nolint:gochecknoglobals
var allowedCycleIntervals = map[int]struct{}{
	1: {}, 5: {}, 15: {}, 30: {}, 60: {}, 120: {}, 180: {}, 240: {},
	360: {}, 480: {}, 720: {}, 1440: {}, 10080: {},
}

// credentialFieldPattern matches field names that typically carry secret
// values returned by third-party Connectors. Used by the read-side stripper
// (FR-028, FR-032, FR-039). The test lowercases the field name, so the
// patterns here only need to be lowercase substrings.
//
//nolint:gochecknoglobals
var credentialFieldPattern = []string{
	"accesstoken",
	"refreshtoken",
	"apikey",
	"api_key",
	"password",
	"secret",
	"authvalue",
	"bearer",
}

func (c *Connector) buildReadRequest(ctx context.Context, params common.ReadParams) (*http.Request, error) {
	url, err := c.readURLForObject(params.ObjectName)
	if err != nil {
		return nil, err
	}

	page := string(params.NextPage)
	if page == "" {
		page = "1"
	}

	url.WithQueryParam(queryParamPage, page)
	url.WithQueryParam(queryParamSize, defaultPerPage)

	return http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
}

// readURLForObject dispatches on ObjectName. Parent-scoped globs are
// recognised by their literal segments (`cycles/{id}/steps`,
// `steps/{id}/parameters`, etc.) and routed to the corresponding /v1.0/ path.
// Everything else falls through to `{BaseURL}/v1.0/{ObjectName}`, which is
// what the literal-name reads (accounts, cycles, templates,
// accountConnectors) want.
func (c *Connector) readURLForObject(objectName string) (*urlbuilder.URL, error) {
	if id, ok := matchParentScoped(objectName, parentPathSegmentCycles, parentChildSegmentCycleSteps); ok {
		return c.buildURL(parentPathSegmentCycles, id, parentChildSegmentCycleSteps)
	}

	if id, ok := matchParentScoped(objectName, parentPathSegmentSteps, parentChildSegmentStepParameters); ok {
		return c.buildURL(parentPathSegmentSteps, id, parentChildSegmentStepParameters)
	}

	if id, ok := matchParentScoped(objectName, parentPathSegmentSteps, parentChildSegmentStepFieldMappings); ok {
		return c.buildURL(parentPathSegmentSteps, id, parentChildSegmentStepFieldMappings)
	}

	if id, ok := matchParentScoped(objectName, parentPathSegmentSteps, parentChildSegmentPrerequisites); ok {
		return c.buildURL(parentPathSegmentSteps, id, parentChildSegmentPrerequisites)
	}

	if objectName == objectNameAccountConnectors {
		return c.buildURL("account", "connectors")
	}

	return c.buildURL(objectName)
}

// matchParentScoped reports whether objectName fits the `{parent}/{id}/{child}`
// shape and, if so, returns the id. Example:
//
//	matchParentScoped("cycles/abc/steps", "cycles", "steps") → "abc", true
func matchParentScoped(objectName, parent, child string) (string, bool) {
	segments := strings.Split(objectName, "/")
	if len(segments) != 3 {
		return "", false
	}

	if segments[0] != parent || segments[2] != child {
		return "", false
	}

	if segments[1] == "" {
		return "", false
	}

	return segments[1], true
}

func (c *Connector) parseReadResponse(
	ctx context.Context,
	params common.ReadParams,
	request *http.Request,
	resp *common.JSONHTTPResponse,
) (*common.ReadResult, error) {
	currentPage := readCurrentPage(request)

	return common.ParseResult(
		resp,
		recordsFromRoot,
		nextPageFromTotalHeader(resp, currentPage),
		common.MakeMarshaledDataFunc(stripCredentialLikeFields),
		params.Fields,
	)
}

func recordsFromRoot(node *ajson.Node) ([]*ajson.Node, error) {
	if node == nil {
		return nil, nil
	}

	if node.IsArray() {
		return node.GetArray()
	}

	if node.IsObject() {
		return []*ajson.Node{node}, nil
	}

	return nil, nil
}

// stripCredentialLikeFields is a RecordTransformer applied to every record
// surfaced by parseReadResponse. It walks the object-to-map conversion of
// the record and, for every top-level key whose name matches the credential
// heuristic (see credentialFieldPattern), replaces the value with the empty
// string. `Raw` on the ReadResultRow is populated separately from the node
// itself so it keeps Cyclr's response verbatim (FR-051).
func stripCredentialLikeFields(node *ajson.Node) (map[string]any, error) {
	record, err := jsonquery.Convertor.ObjectToMap(node)
	if err != nil {
		return nil, err
	}

	for key := range record {
		if looksLikeCredentialFieldName(key) {
			record[key] = ""
		}
	}

	return record, nil
}

func looksLikeCredentialFieldName(name string) bool {
	lower := strings.ToLower(name)
	for _, pattern := range credentialFieldPattern {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

func nextPageFromTotalHeader(resp *common.JSONHTTPResponse, currentPage int) common.NextPageFunc {
	return func(_ *ajson.Node) (string, error) {
		total := 0

		if raw := resp.Headers.Get(headerTotalPage); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err == nil {
				total = parsed
			}
		}

		if total <= 0 || currentPage >= total {
			return "", nil
		}

		return strconv.Itoa(currentPage + 1), nil
	}
}

func readCurrentPage(request *http.Request) int {
	if request == nil || request.URL == nil {
		return 1
	}

	raw := request.URL.Query().Get(queryParamPage)
	if raw == "" {
		return 1
	}

	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 {
		return 1
	}

	return page
}

// buildWriteRequest routes the Account-scope writes:
//  1. `cycles` (no RecordId) with TemplateId in RecordData → install-from-template.
//  2. `cycles:activate` / `cycles:deactivate` → cycle state transitions.
//  3. `stepParameters` / `stepFieldMappings` (with RecordId) → step-input update.
//  4. `accountConnectors` (no RecordId) → install a catalog Connector.
func (c *Connector) buildWriteRequest(ctx context.Context, params common.WriteParams) (*http.Request, error) {
	switch {
	case strings.HasSuffix(params.ObjectName, ":"+pathSegmentActivate):
		return c.buildCycleActivateRequest(ctx, params)
	case strings.HasSuffix(params.ObjectName, ":"+pathSegmentDeactivate):
		return c.buildCycleActionRequest(ctx, params, pathSegmentDeactivate, nil)
	case params.ObjectName == objectNameCycles && params.RecordId == "":
		return c.buildInstallFromTemplateRequest(ctx, params)
	case params.ObjectName == objectNameStepParameters && params.RecordId != "":
		return c.buildStepInputUpdateRequest(ctx, params, parentChildSegmentStepParameters)
	case params.ObjectName == objectNameStepFieldMappings && params.RecordId != "":
		return c.buildStepInputUpdateRequest(ctx, params, parentChildSegmentStepFieldMappings)
	case params.ObjectName == objectNameAccountConnectors && params.RecordId == "":
		return c.buildAccountConnectorInstallRequest(ctx, params)
	}

	return nil, fmt.Errorf("%w: cyclraccount does not support %s with RecordId=%q",
		common.ErrOperationNotSupportedForObject, params.ObjectName, params.RecordId)
}

func (c *Connector) buildInstallFromTemplateRequest(
	ctx context.Context,
	params common.WriteParams,
) (*http.Request, error) {
	templateID, err := extractStringField(params.RecordData, payloadKeyTemplateId)
	if err != nil {
		return nil, fmt.Errorf("install-from-template: %w", err)
	}

	url, err := c.buildURL("templates", templateID, pathSegmentInstall)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create install-from-template request: %w", err)
	}

	return req, nil
}

func (c *Connector) buildCycleActivateRequest(
	ctx context.Context,
	params common.WriteParams,
) (*http.Request, error) {
	payload, err := buildActivatePayload(params.RecordData)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal activate payload: %w", err)
	}

	return c.buildCycleActionRequest(ctx, params, pathSegmentActivate, bytes.NewReader(body))
}

// buildCycleActionRequest issues PUT /v1.0/cycles/{RecordId}/{action}. body may
// be nil (empty body).
func (c *Connector) buildCycleActionRequest(
	ctx context.Context,
	params common.WriteParams,
	action string,
	body *bytes.Reader,
) (*http.Request, error) {
	if params.RecordId == "" {
		return nil, fmt.Errorf("%w: cycles:%s requires RecordId", common.ErrMissingRecordID, action)
	}

	url, err := c.buildURL(objectNameCycles, params.RecordId, action)
	if err != nil {
		return nil, err
	}

	var reader *bytes.Reader
	if body != nil {
		reader = body
	}

	var req *http.Request

	if reader == nil {
		req, err = http.NewRequestWithContext(ctx, http.MethodPut, url.String(), nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodPut, url.String(), reader)
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create cycles:%s request: %w", action, err)
	}

	return req, nil
}

// buildStepInputUpdateRequest issues:
//
//	PUT /v1.0/steps/{StepId}/parameters/{parameterId}
//	PUT /v1.0/steps/{StepId}/fieldmappings/{fieldId}
//
// StepId is extracted from RecordData and stripped from the outbound body.
// Only the mapping shape (MappingType + Value / SourceStepId+SourceFieldName /
// VariableName) is forwarded (FR-035..039). Unknown MappingType values pass
// through uninterpreted (FR-037).
func (c *Connector) buildStepInputUpdateRequest(
	ctx context.Context,
	params common.WriteParams,
	childSegment string,
) (*http.Request, error) {
	stepID, err := extractStringField(params.RecordData, payloadKeyStepId)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", childSegment, err)
	}

	record, err := recordDataAsMap(params.RecordData)
	if err != nil {
		return nil, err
	}

	// Outbound body carries only the mapping shape — not StepId, not anything
	// else we don't recognise. This keeps the wire payload stable and keeps
	// arbitrary fields from leaking into error context (FR-039).
	body := map[string]any{}
	for _, key := range []string{
		payloadKeyMappingType,
		payloadKeyValue,
		payloadKeySourceStepId,
		payloadKeySourceFieldName,
		payloadKeyVariableName,
	} {
		if v, ok := record[key]; ok && v != nil {
			body[key] = v
		}
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s update body: %w", childSegment, err)
	}

	url, err := c.buildURL(parentPathSegmentSteps, stepID, childSegment, params.RecordId)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url.String(), bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("failed to create %s update request: %w", childSegment, err)
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// buildAccountConnectorInstallRequest issues POST /v1.0/connectors/{ConnectorId}/install
// with body { Name, Description, AuthValue }. AuthValue is the caller's
// plain-text credential for API-key / Basic-auth Connectors; we forward it
// to Cyclr and never echo it back anywhere in this package (FR-034).
func (c *Connector) buildAccountConnectorInstallRequest(
	ctx context.Context,
	params common.WriteParams,
) (*http.Request, error) {
	connectorID, err := extractStringField(params.RecordData, payloadKeyConnectorId)
	if err != nil {
		return nil, fmt.Errorf("accountConnectors install: %w", err)
	}

	record, err := recordDataAsMap(params.RecordData)
	if err != nil {
		return nil, err
	}

	body := map[string]any{}
	for _, key := range []string{payloadKeyName, payloadKeyDescription, payloadKeyAuthValue} {
		if v, ok := record[key]; ok && v != nil {
			body[key] = v
		}
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		// Deliberately do NOT include `body` or `record` in this error —
		// AuthValue may be inside.
		return nil, fmt.Errorf("failed to marshal accountConnectors install body: %w", err)
	}

	url, err := c.buildURL("connectors", connectorID, pathSegmentInstall)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("failed to create accountConnectors install request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// buildActivatePayload — see earlier version. Extracts only activate-relevant
// fields and validates Interval against the closed allowed set.
func buildActivatePayload(data any) (map[string]any, error) {
	payload := make(map[string]any, 3)

	record, err := recordDataAsMap(data)
	if err != nil {
		return nil, err
	}

	if intervalRaw, ok := record[payloadKeyInterval]; ok {
		interval, err := coerceInt(intervalRaw)
		if err != nil {
			return nil, fmt.Errorf("%w: Interval must be an integer", common.ErrBadRequest)
		}

		if _, ok := allowedCycleIntervals[interval]; !ok {
			return nil, fmt.Errorf("%w: Interval=%d is not in the allowed set {1,5,15,30,60,120,180,240,360,480,720,1440,10080}",
				common.ErrBadRequest, interval)
		}

		payload[payloadKeyInterval] = interval
	}

	if startTime, ok := record[payloadKeyStartTime]; ok && startTime != nil {
		payload[payloadKeyStartTime] = startTime
	}

	if runOnce, ok := record[payloadKeyRunOnce]; ok && runOnce != nil {
		payload[payloadKeyRunOnce] = runOnce
	}

	return payload, nil
}

func (c *Connector) parseWriteResponse(
	ctx context.Context,
	params common.WriteParams,
	request *http.Request,
	resp *common.JSONHTTPResponse,
) (*common.WriteResult, error) {
	body, ok := resp.Body()
	if !ok {
		return &common.WriteResult{
			Success:  true,
			RecordId: params.RecordId,
		}, nil
	}

	recordID, err := jsonquery.New(body).StringOptional("Id")
	if err == nil && recordID != nil && *recordID != "" {
		return &common.WriteResult{
			Success:  true,
			RecordId: *recordID,
		}, nil
	}

	return &common.WriteResult{
		Success:  true,
		RecordId: params.RecordId,
	}, nil
}

func (c *Connector) buildDeleteRequest(ctx context.Context, params common.DeleteParams) (*http.Request, error) {
	if params.ObjectName != objectNameCycles {
		return nil, fmt.Errorf("%w: %s", common.ErrOperationNotSupportedForObject, params.ObjectName)
	}

	if params.RecordId == "" {
		return nil, common.ErrMissingRecordID
	}

	url, err := c.buildURL(params.ObjectName, params.RecordId)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, http.MethodDelete, url.String(), nil)
}

func (c *Connector) parseDeleteResponse(
	ctx context.Context,
	params common.DeleteParams,
	request *http.Request,
	resp *common.JSONHTTPResponse,
) (*common.DeleteResult, error) {
	if resp.Code != http.StatusOK && resp.Code != http.StatusNoContent {
		return nil, fmt.Errorf("%w: cyclrAccount delete returned status %d", common.ErrRequestFailed, resp.Code)
	}

	return &common.DeleteResult{Success: true}, nil
}

// --- helpers -----------------------------------------------------------------

func recordDataAsMap(data any) (map[string]any, error) {
	if data == nil {
		return map[string]any{}, nil
	}

	if m, ok := data.(map[string]any); ok {
		return m, nil
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("record data must be a JSON object: %w", err)
	}

	result := make(map[string]any)
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("record data must be a JSON object: %w", err)
	}

	return result, nil
}

func extractStringField(data any, key string) (string, error) {
	record, err := recordDataAsMap(data)
	if err != nil {
		return "", err
	}

	value, ok := record[key]
	if !ok {
		return "", fmt.Errorf("%w: missing required field %q", common.ErrBadRequest, key)
	}

	str, ok := value.(string)
	if !ok || str == "" {
		return "", fmt.Errorf("%w: field %q must be a non-empty string", common.ErrBadRequest, key)
	}

	return str, nil
}

func coerceInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, err
		}

		return int(n), nil
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, err
		}

		return n, nil
	default:
		return 0, fmt.Errorf("cannot coerce %T to int", value)
	}
}
