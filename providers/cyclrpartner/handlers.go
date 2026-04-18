package cyclrpartner

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

	// pathSegmentSuspend / pathSegmentResume are appended to the Account URL
	// for the synthetic action writes routed through object names
	// `accounts:suspend` and `accounts:resume`.
	pathSegmentSuspend = "suspend"
	pathSegmentResume  = "resume"
)

// buildReadRequest constructs a list-accounts request. Cyclr's typed
// library surface is list-only for reads; single-record reads (e.g.
// `GET /v1.0/accounts/{id}`) are exposed via pass-through (Support.Proxy)
// because common.ReadParams does not carry a RecordId.
func (c *Connector) buildReadRequest(ctx context.Context, params common.ReadParams) (*http.Request, error) {
	url, err := c.buildURL(params.ObjectName)
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

// parseReadResponse walks the bare-array shape Cyclr emits on list endpoints
// and pages forward via the Total-Pages response header (research §5). Exact
// header name is confirmed at Layer-2 (research §12.1); if it differs we flip
// the `headerTotalPage` constant above and add the alternative to
// readTotalPagesFromResponse.
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
		common.MakeMarshaledDataFunc(nil),
		params.Fields,
	)
}

// recordsFromRoot handles both the bare-array shape (most Cyclr list
// endpoints) and a single-object shape (a defensive fallback, in case Layer-2
// reveals a wrapper key). Layer-2 will narrow this to one path.
func recordsFromRoot(node *ajson.Node) ([]*ajson.Node, error) {
	if node == nil {
		return nil, nil
	}

	if node.IsArray() {
		return node.GetArray()
	}

	// Single object returned (non-paginated) — wrap into a single-element slice
	// so ParseResult's downstream marshaling treats it uniformly.
	if node.IsObject() {
		return []*ajson.Node{node}, nil
	}

	return nil, nil
}

// nextPageFromTotalHeader returns a NextPageFunc that reads the Total-Pages
// response header and, if the current page hasn't reached it, yields the next
// page number as the opaque next-page token. Cyclr does not emit an RFC 5988
// Link header (research §5), so we must compute the next page ourselves.
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

// buildWriteRequest routes the Account create / update paths.
//
// `accounts:suspend` / `accounts:resume` were part of the original contract
// but Cyclr's public Partner API does not expose those routes (Layer-2
// verification returned 404 at every probed variant). Routing for them is
// retained here as a not-supported stub so callers get a clear typed error.
func (c *Connector) buildWriteRequest(ctx context.Context, params common.WriteParams) (*http.Request, error) {
	switch {
	case strings.HasSuffix(params.ObjectName, ":"+pathSegmentSuspend),
		strings.HasSuffix(params.ObjectName, ":"+pathSegmentResume):
		return nil, fmt.Errorf("%w: %s — Cyclr does not expose a Partner suspend/resume endpoint at the assumed path; see providers/cyclrpartner/objects.go",
			common.ErrOperationNotSupportedForObject, params.ObjectName)
	}

	// Plain create / update paths.
	url, err := c.accountWriteURL(params.ObjectName, params.RecordId)
	if err != nil {
		return nil, err
	}

	method := http.MethodPost
	if params.RecordId != "" {
		method = http.MethodPut
	}

	body, err := marshalRecordData(params.RecordData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create cyclrPartner write request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func (c *Connector) accountWriteURL(objectName, recordID string) (*urlbuilder.URL, error) {
	if objectName != objectNameAccounts {
		return nil, fmt.Errorf("%w: %s", common.ErrOperationNotSupportedForObject, objectName)
	}

	if recordID == "" {
		return c.buildURL(objectName)
	}

	return c.buildURL(objectName, recordID)
}

func marshalRecordData(data any) ([]byte, error) {
	if data == nil {
		return []byte("{}"), nil
	}

	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cyclrPartner record data: %w", err)
	}

	return body, nil
}

// parseWriteResponse extracts the Cyclr-assigned `Id` from create responses
// and echoes back the caller's `RecordId` for updates / suspend / resume
// where Cyclr may return either the full Account object or a 204-style empty
// body.
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

	// If the response is a JSON object with an Id field, surface it as the
	// RecordId. Create responses emit the new Account; action endpoints
	// (suspend/resume) typically do not.
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

// buildDeleteRequest issues `DELETE /v1.0/accounts/{RecordId}`. Cyclr returns
// 204 No Content on success; delete-refusal (e.g., Cycles still exist) comes
// back as a 4xx with a populated `Message` which the error interpreter
// surfaces verbatim.
func (c *Connector) buildDeleteRequest(ctx context.Context, params common.DeleteParams) (*http.Request, error) {
	if params.ObjectName != objectNameAccounts {
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
		return nil, fmt.Errorf("%w: cyclrPartner delete returned status %d", common.ErrRequestFailed, resp.Code)
	}

	return &common.DeleteResult{Success: true}, nil
}
