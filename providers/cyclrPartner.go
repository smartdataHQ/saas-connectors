package providers

const CyclrPartner Provider = "cyclrPartner"

// Cyclr is an embedded-iPaaS platform. The Partner-scope provider addresses
// Partner-level resources (Accounts, catalog Templates, catalog Connectors).
// Account-scoped operations (Cycles, Cycle Steps, installed Connectors, etc.)
// live in the companion `cyclrAccount` provider.
//
// Cyclr runs several regional API instances (api.cyclr.com, api.eu.cyclr.com,
// api.us2.cyclr.com, api.cyclr.uk, or a private instance). The specific host
// is selected per-connection via the `apiDomain` metadata input.
//
//nolint:lll
func init() {
	SetInfo(CyclrPartner, ProviderInfo{
		DisplayName: "Cyclr (Partner)",
		AuthType:    Oauth2,
		BaseURL:     "https://{{.apiDomain}}",
		Oauth2Opts: &Oauth2Opts{
			GrantType:                 ClientCredentials,
			TokenURL:                  "https://{{.apiDomain}}/oauth/token",
			ExplicitScopesRequired:    false,
			ExplicitWorkspaceRequired: false,
		},
		Support: Support{
			BulkWrite: BulkWriteSupport{
				Insert: false,
				Update: false,
				Upsert: false,
				Delete: false,
			},
			Proxy:     true,
			Read:      false,
			Write:     false,
			Subscribe: false,
		},
		Metadata: &ProviderMetadata{
			Input: []MetadataItemInput{
				{
					Name:        "apiDomain",
					DisplayName: "Cyclr API Domain",
					DocsURL:     "https://community.cyclr.com/user-documentation/api/introduction-to-the-cyclr-api",
					Prompt:      "Your Cyclr API host without protocol, e.g. api.cyclr.com, api.eu.cyclr.com, api.us2.cyclr.com, api.cyclr.uk, or a private instance host.",
				},
			},
		},
	})
}
