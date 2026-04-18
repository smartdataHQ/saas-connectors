// Package cyclraccount implements the deep connector for the Cyclr Account
// API scope (Cycles, installed Connectors, Steps, etc. within a single
// customer Account).
//
// Authentication is OAuth 2.0 Client Credentials with a token scope of
// `account:{accountApiId}`. On top of the OAuth2 bearer, every outbound HTTP
// request carries an `X-Cyclr-Account: {accountApiId}` header, which the
// connector attaches by wrapping the caller-provided AuthenticatedHTTPClient
// with accountHeaderClient. Because the wrapping happens before
// components.Initialize constructs the Reader / Writer / Deleter, both typed
// deep calls and proxy pass-through calls share the wrapped client — a caller
// routing a raw request through the proxy cannot forge a different Account
// context (FR-042).
package cyclraccount

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/interpreter"
	"github.com/amp-labs/connectors/internal/components"
	"github.com/amp-labs/connectors/internal/components/deleter"
	"github.com/amp-labs/connectors/internal/components/operations"
	"github.com/amp-labs/connectors/internal/components/reader"
	"github.com/amp-labs/connectors/internal/components/schema"
	"github.com/amp-labs/connectors/internal/components/writer"
	"github.com/amp-labs/connectors/providers"
)

const (
	// metadataKeyAccountApiId is the metadata-input key on the providers.CyclrAccount
	// ProviderInfo that carries the Cyclr Account API ID this connection targets.
	metadataKeyAccountApiId = "accountApiId"

	// metadataKeyAPIDomain is the metadata-input key carrying the Cyclr API
	// host (e.g., "api.cyclr.com"). Used as the single-element allowlist the
	// accountHeaderClient enforces on every outbound request (FR-041).
	metadataKeyAPIDomain = "apiDomain"

	// headerAccountApiId is the Cyclr-defined request header that selects the
	// Account a given API call operates against. Every request issued by a
	// cyclrAccount connection carries it.
	headerAccountApiId = "X-Cyclr-Account"
)

// ErrMissingAccountApiId indicates a cyclrAccount connection was constructed
// without metadata.accountApiId populated. It must be present — without it we
// cannot build the X-Cyclr-Account header or the OAuth2 `account:<uuid>`
// scope.
var ErrMissingAccountApiId = errors.New("cyclrAccount connection requires metadata." + metadataKeyAccountApiId)

// ErrHostNotAllowed is returned when a request attempts to reach a host
// outside the configured Cyclr apiDomain (FR-041). Pass-through callers
// cannot smuggle requests to arbitrary hosts using a cyclrAccount connection.
var ErrHostNotAllowed = errors.New("request host not in cyclrAccount allowlist")

type Connector struct {
	*components.Connector

	common.RequireAuthenticatedClient

	components.SchemaProvider
	components.Reader
	components.Writer
	components.Deleter

	accountApiId string
}

func NewConnector(params common.ConnectorParams) (*Connector, error) {
	accountApiId := ""
	apiDomain := ""

	if params.Metadata != nil {
		accountApiId = params.Metadata[metadataKeyAccountApiId]
		apiDomain = params.Metadata[metadataKeyAPIDomain]
	}

	if accountApiId == "" {
		return nil, ErrMissingAccountApiId
	}

	// Wrap the authenticated client BEFORE components.Initialize wires it into
	// Transport. Doing it here (rather than inside constructor) ensures the
	// same wrapped client flows into Reader / Writer / Deleter and into any
	// pass-through caller that pulls c.HTTPClient().Client later.
	if params.AuthenticatedClient != nil {
		params.AuthenticatedClient = &accountHeaderClient{
			inner:        params.AuthenticatedClient,
			accountApiId: accountApiId,
			allowedHost:  apiDomain,
		}
	}

	conn, err := components.Initialize(providers.CyclrAccount, params, constructor)
	if err != nil {
		return nil, err
	}

	conn.accountApiId = accountApiId

	return conn, nil
}

func constructor(base *components.Connector) (*Connector, error) {
	connector := &Connector{Connector: base}

	registry, err := components.NewEndpointRegistry(supportedOperations())
	if err != nil {
		return nil, err
	}

	connector.SchemaProvider = schema.NewOpenAPISchemaProvider(
		connector.ProviderContext.Module(),
		schemas,
	)

	errorHandler := interpreter.ErrorHandler{
		JSON: interpreter.NewFaultyResponder(errorFormats, statusCodeMapping),
	}.Handle

	connector.Reader = reader.NewHTTPReader(
		connector.HTTPClient().Client,
		registry,
		connector.ProviderContext.Module(),
		operations.ReadHandlers{
			BuildRequest:  connector.buildReadRequest,
			ParseResponse: connector.parseReadResponse,
			ErrorHandler:  errorHandler,
		},
	)

	connector.Writer = writer.NewHTTPWriter(
		connector.HTTPClient().Client,
		registry,
		connector.ProviderContext.Module(),
		operations.WriteHandlers{
			BuildRequest:  connector.buildWriteRequest,
			ParseResponse: connector.parseWriteResponse,
			ErrorHandler:  errorHandler,
		},
	)

	connector.Deleter = deleter.NewHTTPDeleter(
		connector.HTTPClient().Client,
		registry,
		connector.ProviderContext.Module(),
		operations.DeleteHandlers{
			BuildRequest:  connector.buildDeleteRequest,
			ParseResponse: connector.parseDeleteResponse,
			ErrorHandler:  errorHandler,
		},
	)

	return connector, nil
}

// accountHeaderClient wraps an AuthenticatedHTTPClient and sets
// X-Cyclr-Account: {accountApiId} on every outbound request. Any
// caller-supplied value for that header is overwritten via Header.Set, which
// preserves Account-scope isolation under proxy pass-through (FR-042).
//
// Additionally, if allowedHost is non-empty, every request's target Host is
// validated against it; any host that does not match is refused with
// ErrHostNotAllowed. This prevents pass-through callers from smuggling
// requests to arbitrary hosts using a cyclrAccount connection (FR-041).
// An empty allowedHost disables the check — used by unit tests against
// mockserver, which the test harness sets via SetBaseURL after construction.
type accountHeaderClient struct {
	inner        common.AuthenticatedHTTPClient
	accountApiId string
	allowedHost  string
}

func (c *accountHeaderClient) Do(req *http.Request) (*http.Response, error) {
	if c.allowedHost != "" && !hostMatches(req, c.allowedHost) {
		return nil, fmt.Errorf("%w: got %q, allowed %q", ErrHostNotAllowed, requestHost(req), c.allowedHost)
	}

	// Clone to avoid mutating the caller's request headers.
	req = req.Clone(req.Context())
	req.Header.Set(headerAccountApiId, c.accountApiId)

	return c.inner.Do(req)
}

func (c *accountHeaderClient) CloseIdleConnections() {
	c.inner.CloseIdleConnections()
}

// requestHost returns the target host for an outbound request, lowercased and
// with any port stripped. net/http uses req.URL.Host on client-side requests.
func requestHost(req *http.Request) string {
	host := ""
	if req != nil && req.URL != nil {
		host = req.URL.Host
	}

	if host == "" && req != nil {
		host = req.Host
	}

	// Strip :<port> if present.
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}

	return strings.ToLower(host)
}

func hostMatches(req *http.Request, allowed string) bool {
	return requestHost(req) == strings.ToLower(allowed)
}
