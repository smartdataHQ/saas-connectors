package cyclrpartner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoHardcodedCyclrHost exercises FR-002: every handler composes outbound
// URLs from c.ProviderInfo().BaseURL (itself resolved from metadata.apiDomain)
// rather than hard-coding a Cyclr region. If this test fails, a handler has
// baked in an absolute URL — callers targeting non-default regions would be
// misrouted and pass-through's allowlist would be bypassed.
func TestNoHardcodedCyclrHost(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"handlers.go", "url.go"} {
		file := file
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Join(".", file))
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}

			content := string(data)
			for _, forbidden := range []string{
				"api.cyclr.com",
				"api.eu.cyclr.com",
				"api.us2.cyclr.com",
				"api.cyclr.uk",
			} {
				if strings.Contains(content, forbidden) {
					t.Errorf("%s contains hardcoded Cyclr host %q — URLs must derive from c.ProviderInfo().BaseURL",
						file, forbidden)
				}
			}
		})
	}
}
