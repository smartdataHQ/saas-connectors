// Package cyclrpartner contains the Layer-2 test harness for the
// providers.CyclrPartner deep connector. Construct the connector via
// GetCyclrPartnerConnector; it loads creds from
// ~/.ampersand-creds/cyclrPartner.json and wires an OAuth2 Client Credentials
// HTTP client against the Cyclr region identified by metadata.apiDomain.
package cyclrpartner

import (
	"context"
	"fmt"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/scanning/credscanning"
	"github.com/amp-labs/connectors/providers"
	"github.com/amp-labs/connectors/providers/cyclrpartner"
	"github.com/amp-labs/connectors/test/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// fieldAPIDomain is the Cyclr-specific credential field for the API host
// (e.g., `api.cyclr.com`, `api.eu.cyclr.com`). It lives in the creds JSON
// under `metadata.apiDomain` — the same key the runtime ProviderInfo reads.
//
//nolint:gochecknoglobals
var fieldAPIDomain = credscanning.Field{
	Name:      "apiDomain",
	PathJSON:  "metadata.apiDomain",
	SuffixENV: "API_DOMAIN",
}

// GetCyclrPartnerConnector returns a Partner-scope Cyclr connector backed by
// the caller's Layer-2 credentials. Failure to load creds is fatal — this
// harness is only used by ./test/cyclrPartner/{metadata,read,write,delete}
// entrypoints, all of which treat missing creds as a hard stop.
func GetCyclrPartnerConnector(ctx context.Context) *cyclrpartner.Connector {
	filePath := credscanning.LoadPath(providers.CyclrPartner)
	reader := utils.MustCreateProvCredJSON(filePath, false, fieldAPIDomain)

	apiDomain := reader.Get(fieldAPIDomain)
	if apiDomain == "" {
		utils.Fail("cyclrPartner creds missing metadata.apiDomain")
	}

	cfg := &clientcredentials.Config{
		ClientID:     reader.Get(credscanning.Fields.ClientId),
		ClientSecret: reader.Get(credscanning.Fields.ClientSecret),
		TokenURL:     fmt.Sprintf("https://%s/oauth/token", apiDomain),
		AuthStyle:    oauth2.AuthStyleInParams,
	}

	conn, err := cyclrpartner.NewConnector(common.ConnectorParams{
		AuthenticatedClient: oauth2.NewClient(ctx, cfg.TokenSource(ctx)),
		Metadata: map[string]string{
			"apiDomain": apiDomain,
		},
	})
	if err != nil {
		utils.Fail("error creating connector", "error", err)
	}

	return conn
}
