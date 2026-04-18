package cyclraccount

import (
	"context"
	"testing"
)

func TestListObjectMetadata(t *testing.T) {
	t.Parallel()

	conn, err := constructTestConnector("http://localhost")
	if err != nil {
		t.Fatalf("construct connector: %v", err)
	}

	result, err := conn.ListObjectMetadata(context.Background(), []string{objectNameCycles})
	if err != nil {
		t.Fatalf("ListObjectMetadata: %v", err)
	}

	cycles, ok := result.Result[objectNameCycles]
	if !ok {
		t.Fatalf("expected %q in metadata result; got %+v", objectNameCycles, result.Result)
	}

	if cycles.DisplayName == "" {
		t.Errorf("expected non-empty DisplayName on cycles")
	}

	for _, field := range []string{"Id", "Status", "TemplateId", "Interval"} {
		if _, ok := cycles.FieldsMap[field]; !ok {
			t.Errorf("missing field %q in cycles schema", field)
		}
	}
}
