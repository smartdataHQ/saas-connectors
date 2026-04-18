package cyclraccount

import "github.com/amp-labs/connectors/common/urlbuilder"

// apiVersion is the Cyclr API version segment. See cyclrpartner/url.go for
// rationale (BaseURL carries no version; handlers prepend this).
const apiVersion = "v1.0"

// buildURL composes BaseURL + apiVersion + the caller's path parts. Every
// outbound request targets the configured apiDomain (FR-002, FR-041).
func (c *Connector) buildURL(parts ...string) (*urlbuilder.URL, error) {
	segments := make([]string, 0, len(parts)+1)
	segments = append(segments, apiVersion)
	segments = append(segments, parts...)

	return urlbuilder.New(c.ProviderInfo().BaseURL, segments...)
}
