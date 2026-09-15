package jsonquery

import (
	"errors"
	"testing"
)

func TestNestedNullOptionalAndRequired(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		body string
		zoom []string
	}{
		{name: "root", body: `null`},
		{name: "terminal", body: `{"next_page":null}`, zoom: []string{"next_page"}},
		{name: "intermediate", body: `{"next_page":null}`, zoom: []string{"next_page", "links"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			node := helperCreateJSON(t, test.body)
			optional, err := New(node, test.zoom...).StringOptional("uri")
			if err != nil || optional != nil {
				t.Fatalf("optional nested null: value=%v error=%v", optional, err)
			}

			_, err = New(node, test.zoom...).StringRequired("uri")
			if !errors.Is(err, ErrNullJSON) {
				t.Fatalf("required nested null: expected ErrNullJSON, got %v", err)
			}
		})
	}
}
