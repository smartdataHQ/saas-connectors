package providers

const CyclrAccount Provider = "cyclrAccount"

// Cyclr is an embedded-iPaaS platform. The Account-scope provider addresses
// operations that happen inside a single customer Account: Cycles (workflows),
// Cycle Steps, installed Connectors, templates scoped to the Account.
// Partner-level operations (Account CRUD, catalog discovery) live in the
// companion `cyclrPartner` provider.
//
// Scoping:
//   - The `accountApiId` metadata input pins this connection to one Cyclr
//     Account. A single cyclrAccount connection operates against exactly one
//     Account for its lifetime.
//   - At token-request time the OAuth2 scope `account:{accountApiId}` is
//     required (ExplicitScopesRequired:true); the `scopes` value must be
//     populated by the caller with the full `account:<uuid>` string.
//   - At request time the deep connector wraps the authenticated HTTP client
//     with a transport that sets `X-Cyclr-Account: {accountApiId}` on every
//     outbound request. That wiring lives in providers/cyclraccount/.
//
//nolint:lll
func init() {
	SetInfo(CyclrAccount, ProviderInfo{
		DisplayName: "Cyclr (Account)",
		AuthType:    Oauth2,
		BaseURL:     "https://{{.apiDomain}}",
		Oauth2Opts: &Oauth2Opts{
			GrantType:                 ClientCredentials,
			TokenURL:                  "https://{{.apiDomain}}/oauth/token",
			ExplicitScopesRequired:    true,
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
				{
					Name:        "accountApiId",
					DisplayName: "Cyclr Account API ID",
					DocsURL:     "https://community.cyclr.com/user-documentation/api/authorize-account-api-calls",
					Prompt:      "The Account API ID (UUID) this connection operates against. Required both as the OAuth2 scope value (account:<uuid>) and as the X-Cyclr-Account request header on every call.",
				},
			},
		},
	})
}
