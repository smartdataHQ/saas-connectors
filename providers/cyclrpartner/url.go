package cyclrpartner

import "github.com/amp-labs/connectors/common/urlbuilder"

// apiVersion is the Cyclr API version segment. BaseURL on the ProviderInfo
// deliberately omits the version (BEST_PRACTICES.md §16); handlers prepend it
// via buildURL. Keeping the version in exactly one place means a future
// migration to /v2.0 is a single-constant change.
const apiVersion = "v1.0"

// buildURL composes BaseURL + apiVersion + the caller's path parts into a
// URL-safe builder. Handlers use this exclusively so that every outbound
// request targets the configured apiDomain (FR-002, FR-041) rather than
// hard-coded hosts.
func (c *Connector) buildURL(parts ...string) (*urlbuilder.URL, error) {
	segments := make([]string, 0, len(parts)+1)
	segments = append(segments, apiVersion)
	segments = append(segments, parts...)

	return urlbuilder.New(c.ProviderInfo().BaseURL, segments...)
}
