package cyclraccount

import "regexp"

// uuidPattern — see cyclrpartner/utils.go for rationale. Duplicated rather
// than shared per research §4.
//
//nolint:gochecknoglobals
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isUUID reports whether s is a canonical UUID string.
func isUUID(s string) bool {
	return uuidPattern.MatchString(s)
}
