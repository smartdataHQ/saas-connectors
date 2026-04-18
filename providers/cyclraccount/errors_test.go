package cyclraccount

import (
	"errors"
	"strings"
	"testing"
)

// TestCombineErrDoesNotLeakCredentialLikeStrings — see
// cyclrpartner/errors_test.go for rationale (FR-062).
func TestCombineErrDoesNotLeakCredentialLikeStrings(t *testing.T) {
	t.Parallel()

	base := errors.New("base error")

	r := ResponseError{
		Message: "Install failed for connector with access_token=Bearer abc.def.ghi",
	}

	err := r.CombineErr(base)
	text := err.Error()

	for _, forbidden := range []string{"client_secret", "clientSecret="} {
		if strings.Contains(text, forbidden) {
			t.Errorf("error text contains forbidden substring %q: %s", forbidden, text)
		}
	}
}
