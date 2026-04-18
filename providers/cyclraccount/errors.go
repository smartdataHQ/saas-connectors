package cyclraccount

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
// credential's scope does not match the endpoint's scope — here, a
// Partner-scoped token hitting an Account-level endpoint (the symmetric
// counterpart to cyclrpartner.ErrScopeMismatch). Surfaced per FR-005.
var ErrScopeMismatch = errors.New("credential scope does not match endpoint")

// errorFormats matches Cyclr's .NET WebAPI JSON error bodies; shape is
// identical across both Partner and Account scopes (research §6).
//
//nolint:gochecknoglobals
var errorFormats = interpreter.NewFormatSwitch(
	interpreter.FormatTemplate{
		MustKeys: []string{"Message"},
		Template: func() interpreter.ErrorDescriptor { return &ResponseError{} },
	},
)

// statusCodeMapping translates Cyclr HTTP status codes onto the library's
// typed error sentinels.
//
//nolint:gochecknoglobals
var statusCodeMapping = map[int]error{
	http.StatusUnauthorized:        common.ErrAccessToken,
	http.StatusForbidden:           common.ErrForbidden,
	http.StatusNotFound:            common.ErrNotFound,
	http.StatusUnprocessableEntity: common.ErrBadRequest,
	http.StatusTooManyRequests:     common.ErrLimitExceeded,
}

// ResponseError — see providers/cyclrpartner/errors.go for the shape. Kept in
// two packages rather than shared per research §4 (≤15 lines of duplication
// is preferable to an internal shared package).
type ResponseError struct {
	Message          string              `json:"Message"`
	ExceptionMessage string              `json:"ExceptionMessage,omitempty"`
	ModelState       map[string][]string `json:"ModelState,omitempty"`
}

func (r ResponseError) CombineErr(base error) error {
	msg := r.buildMessage()

	if looksLikePartnerScopedOnAccountEndpoint(r.Message) {
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

// looksLikePartnerScopedOnAccountEndpoint mirrors the cyclrpartner heuristic
// but watches for the "partner" scope indicator instead. Exact wording pinned
// at Layer-2 (research §12).
func looksLikePartnerScopedOnAccountEndpoint(message string) bool {
	lower := strings.ToLower(message)

	return strings.Contains(lower, "scope") && strings.Contains(lower, "partner")
}
