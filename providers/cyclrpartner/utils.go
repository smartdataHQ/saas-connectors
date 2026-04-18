package cyclrpartner

import "regexp"

// uuidPattern is a lenient UUID v1–v5 matcher used to short-circuit malformed
// RecordId values before they reach Cyclr. Accepts the canonical hyphenated
// form Cyclr emits.
//
//nolint:gochecknoglobals
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isUUID reports whether s is a canonical UUID string. Handlers can use this
// to reject obviously malformed RecordId values client-side rather than round-
// tripping to Cyclr for the rejection.
func isUUID(s string) bool {
	return uuidPattern.MatchString(s)
}
