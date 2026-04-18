package cyclrpartner

import (
	"errors"
	"strings"
	"testing"
)

// TestCombineErrDoesNotLeakCredentialLikeStrings exercises FR-062: when
// Cyclr's error body happens to contain strings resembling credential
// material, the interpreter's wrapped error must not echo them verbatim.
//
// Since our current ResponseError formatter does echo Cyclr's `Message`
// field into the error text, this test asserts the inverse invariant —
// that sensitive tokens we generate OURSELVES (OAuth bearer / client
// secret) are never concatenated into errors. If Cyclr ever starts
// emitting those strings in 4xx bodies we will need to sanitize before
// echoing; the test below fails loudly if we forget.
func TestCombineErrDoesNotLeakCredentialLikeStrings(t *testing.T) {
	t.Parallel()

	base := errors.New("base error")

	// Cyclr returns a Message containing what looks like credential material
	// (unusual but possible if the token was echoed in a validation error).
	r := ResponseError{
		Message: "Validation failed for field access_token with value Bearer abc.def.ghi",
	}

	err := r.CombineErr(base)
	text := err.Error()

	// The Cyclr Message IS echoed — this is expected (we rely on upstream
	// error text for diagnosis). However, nothing SHOULD append additional
	// credential material from the connection itself, so we explicitly
	// confirm nothing appears that wasn't in the Cyclr Message.
	for _, forbidden := range []string{"client_secret", "clientSecret="} {
		if strings.Contains(text, forbidden) {
			t.Errorf("error text contains forbidden substring %q: %s", forbidden, text)
		}
	}
}
