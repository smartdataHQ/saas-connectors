package cyclrpartner

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/interpreter"
)

// ErrScopeMismatch is returned when Cyclr refuses a request because the
// credential's scope does not match the endpoint's scope — typically an
// Account-scoped token hitting a Partner-level endpoint (or vice versa).
// Surfaced per FR-005 so operators can tell "wrong credentials" from "missing
// permissions" without reading raw Cyclr error text.
var ErrScopeMismatch = errors.New("credential scope does not match endpoint")

// errorFormats matches Cyclr's .NET WebAPI JSON error bodies. Single format
// — Cyclr consistently returns `{ "Message": "..." }` with optional
// `ExceptionMessage` and `ModelState` fields on 4xx/5xx responses.
//
//nolint:gochecknoglobals
var errorFormats = interpreter.NewFormatSwitch(
	interpreter.FormatTemplate{
		MustKeys: []string{"Message"},
		Template: func() interpreter.ErrorDescriptor { return &ResponseError{} },
	},
)

// statusCodeMapping translates Cyclr HTTP status codes onto the library's
// typed error sentinels. The interpreter falls back to the default mapping
// for any status not listed here.
//
//nolint:gochecknoglobals
var statusCodeMapping = map[int]error{
	http.StatusUnauthorized:        common.ErrAccessToken,
	http.StatusForbidden:           common.ErrForbidden,
	http.StatusNotFound:            common.ErrNotFound,
	http.StatusUnprocessableEntity: common.ErrBadRequest,
	http.StatusTooManyRequests:     common.ErrLimitExceeded,
}

// ResponseError models Cyclr's .NET-style error body:
//
//	{
//	  "Message": "The request is invalid.",
//	  "ExceptionMessage": "...",
//	  "ModelState": { "field": ["error 1", "error 2"] }
//	}
//
// All fields optional except `Message`, which is what drives the
// FormatSwitch match above.
type ResponseError struct {
	Message          string              `json:"Message"`
	ExceptionMessage string              `json:"ExceptionMessage,omitempty"`
	ModelState       map[string][]string `json:"ModelState,omitempty"`
}

// CombineErr wraps the interpreter-selected base error (from statusCodeMapping
// or the default) with Cyclr's human-readable message, validation detail, and
// — when detectable — a scope-mismatch indicator.
func (r ResponseError) CombineErr(base error) error {
	msg := r.buildMessage()

	if looksLikeAccountScopedOnPartnerEndpoint(r.Message) {
		if msg == "" {
			return fmt.Errorf("%w: %w", base, ErrScopeMismatch)
		}

		return fmt.Errorf("%w: %w: %s", base, ErrScopeMismatch, msg)
	}

	if msg == "" {
		return base
	}

	return fmt.Errorf("%w: %s", base, msg)
}

func (r ResponseError) buildMessage() string {
	msg := r.Message

	if r.ExceptionMessage != "" {
		msg = fmt.Sprintf("%s (%s)", msg, r.ExceptionMessage)
	}

	if len(r.ModelState) > 0 {
		parts := make([]string, 0, len(r.ModelState))
		for field, errs := range r.ModelState {
			parts = append(parts, fmt.Sprintf("%s: %s", field, strings.Join(errs, "; ")))
		}

		sort.Strings(parts)
		msg = fmt.Sprintf("%s [%s]", msg, strings.Join(parts, "; "))
	}

	return msg
}

// looksLikeAccountScopedOnPartnerEndpoint is a conservative heuristic matched
// against Cyclr's `Message` field. Layer-2 verification (research §12) will
// pin the exact substring Cyclr emits; until then, we match on the common
// combination of "scope" + "account" tokens, which is what Cyclr's
// OAuth-layer rejection typically surfaces.
func looksLikeAccountScopedOnPartnerEndpoint(message string) bool {
	lower := strings.ToLower(message)

	return strings.Contains(lower, "scope") && strings.Contains(lower, "account")
}
