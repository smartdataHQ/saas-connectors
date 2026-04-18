package cyclraccount

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"testing"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/test/utils/mockutils"
)

const testAccountApiId = "11111111-1111-1111-1111-111111111111"

// constructTestConnector returns a Connector whose BaseURL has been rewritten
// to target a mockserver instance. Shared by read_test, write_test, delete_test.
//
// After SetBaseURL, the accountHeaderClient's allowedHost is cleared so the
// FR-041 host allowlist does not reject requests to the mockserver. The
// allowlist is exercised directly by TestHostAllowlistRefusesOutsideHost.
func constructTestConnector(serverURL string) (*Connector, error) {
	conn, err := NewConnector(common.ConnectorParams{
		Module:              common.ModuleRoot,
		AuthenticatedClient: mockutils.NewClient(),
		Metadata: map[string]string{
			"apiDomain":    "api.cyclr.com",
			"accountApiId": testAccountApiId,
		},
	})
	if err != nil {
		return nil, err
	}

	conn.SetBaseURL(mockutils.ReplaceURLOrigin(conn.HTTPClient().Base, serverURL))

	if client, ok := conn.HTTPClient().Client.(*accountHeaderClient); ok {
		client.allowedHost = ""
	}

	return conn, nil
}

// TestMissingAccountApiIdFailsFast exercises FR-004: a cyclrAccount
// connection cannot be constructed without metadata.accountApiId — the
// resulting error clearly identifies the cause.
func TestMissingAccountApiIdFailsFast(t *testing.T) {
	t.Parallel()

	_, err := NewConnector(common.ConnectorParams{
		Module:              common.ModuleRoot,
		AuthenticatedClient: mockutils.NewClient(),
		Metadata: map[string]string{
			"apiDomain": "api.cyclr.com",
		},
	})
	if err == nil {
		t.Fatalf("expected construction to fail without accountApiId")
	}

	if err.Error() == "" || err != ErrMissingAccountApiId {
		t.Errorf("expected ErrMissingAccountApiId, got %v", err)
	}
}

// TestAccountHeaderOverridesCallerSuppliedValue exercises FR-042: the
// account-scope header is set by the transport regardless of what callers
// attempt to inject, preventing forged Account context under pass-through.
func TestAccountHeaderOverridesCallerSuppliedValue(t *testing.T) {
	t.Parallel()

	var observed string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed = r.Header.Get(headerAccountApiId)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := &accountHeaderClient{
		inner:        mockutils.NewClient(),
		accountApiId: testAccountApiId,
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("construct request: %v", err)
	}

	// Caller forges an Account header — the transport MUST overwrite it.
	req.Header.Set(headerAccountApiId, "attacker-supplied")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	_ = resp.Body.Close()

	if observed != testAccountApiId {
		t.Errorf("expected X-Cyclr-Account=%q, observed=%q", testAccountApiId, observed)
	}
}

// TestHostAllowlistRefusesOutsideHost exercises FR-041: requests targeting
// hosts outside the configured apiDomain are refused before hitting the wire.
func TestHostAllowlistRefusesOutsideHost(t *testing.T) {
	t.Parallel()

	client := &accountHeaderClient{
		inner:        mockutils.NewClient(),
		accountApiId: testAccountApiId,
		allowedHost:  "api.cyclr.com",
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://evil.example.com/v1.0/accounts", nil)
	if err != nil {
		t.Fatalf("construct request: %v", err)
	}

	_, err = client.Do(req)
	if err == nil {
		t.Fatalf("expected ErrHostNotAllowed, got nil")
	}

	if !errors.Is(err, ErrHostNotAllowed) {
		t.Errorf("expected ErrHostNotAllowed, got %v", err)
	}
}

// TestHostAllowlistAllowsConfiguredHost confirms the allowlist does not
// interfere with legitimate requests to the configured apiDomain.
func TestHostAllowlistAllowsConfiguredHost(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	parsed, err := neturl.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	client := &accountHeaderClient{
		inner:        mockutils.NewClient(),
		accountApiId: testAccountApiId,
		allowedHost:  parsed.Hostname(),
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("construct request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	_ = resp.Body.Close()
}
