package cyclrpartner

import (
	"context"
	"testing"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/test/utils/mockutils"
)

func TestListObjectMetadata(t *testing.T) {
	t.Parallel()

	conn, err := NewConnector(common.ConnectorParams{
		Module:              common.ModuleRoot,
		AuthenticatedClient: mockutils.NewClient(),
		Metadata: map[string]string{
			"apiDomain": "api.cyclr.com",
		},
	})
	if err != nil {
		t.Fatalf("construct connector: %v", err)
	}

	t.Run("accounts schema is present and exposes Id/Name", func(t *testing.T) {
		t.Parallel()

		result, err := conn.ListObjectMetadata(context.Background(), []string{objectNameAccounts})
		if err != nil {
			t.Fatalf("ListObjectMetadata: %v", err)
		}

		accounts, ok := result.Result[objectNameAccounts]
		if !ok {
			t.Fatalf("expected %q in metadata result; got %+v", objectNameAccounts, result.Result)
		}

		if accounts.DisplayName == "" {
			t.Errorf("expected non-empty DisplayName on accounts")
		}

		for _, field := range []string{"Id", "Name", "Timezone", "CreatedDate"} {
			if _, ok := accounts.FieldsMap[field]; !ok {
				t.Errorf("missing field %q in accounts schema", field)
			}
		}
	})

	t.Run("templates schema is present", func(t *testing.T) {
		t.Parallel()

		result, err := conn.ListObjectMetadata(context.Background(), []string{"templates"})
		if err != nil {
			t.Fatalf("ListObjectMetadata: %v", err)
		}

		templates, ok := result.Result["templates"]
		if !ok {
			t.Fatalf("expected 'templates' in metadata result")
		}

		if _, ok := templates.FieldsMap["Id"]; !ok {
			t.Errorf("templates schema missing Id field")
		}
	})

	t.Run("unknown objects surface a typed error", func(t *testing.T) {
		t.Parallel()

		result, err := conn.ListObjectMetadata(context.Background(), []string{"nonexistent"})
		if err != nil {
			// Some providers return the error via result.Errors; either is
			// acceptable here — we just want to know the unknown object was
			// rejected in a typed way.
			return
		}

		if _, ok := result.Errors["nonexistent"]; !ok {
			t.Errorf("expected error for unknown object; got %+v", result)
		}
	})
}
