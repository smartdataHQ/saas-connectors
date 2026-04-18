// Package cyclraccount contains the Layer-2 test harness for the
// providers.CyclrAccount deep connector. Construct the connector via
// GetCyclrAccountConnector; it loads creds from
// ~/.ampersand-creds/cyclrAccount.json, validates the required accountApiId,
// and wires an OAuth2 Client Credentials HTTP client with
// scope=account:{accountApiId} against the Cyclr region identified by
// metadata.apiDomain.
package cyclraccount

import (
	"context"
	"fmt"

	"github.com/amp-labs/connectors/common"
	"github.com/amp-labs/connectors/common/scanning/credscanning"
	"github.com/amp-labs/connectors/providers"
	"github.com/amp-labs/connectors/providers/cyclraccount"
	"github.com/amp-labs/connectors/test/utils"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

//nolint:gochecknoglobals
var fieldAPIDomain = credscanning.Field{
	Name:      "apiDomain",
	PathJSON:  "metadata.apiDomain",
	SuffixENV: "API_DOMAIN",
}

//nolint:gochecknoglobals
var fieldAccountApiId = credscanning.Field{
	Name:      "accountApiId",
	PathJSON:  "metadata.accountApiId",
	SuffixENV: "ACCOUNT_API_ID",
}

// GetCyclrAccountConnector returns an Account-scope Cyclr connector backed by
// the caller's Layer-2 credentials. Fails fast on missing metadata.apiDomain
// or metadata.accountApiId — without the latter we cannot build the OAuth2
// `account:<uuid>` scope or the X-Cyclr-Account header.
func GetCyclrAccountConnector(ctx context.Context) *cyclraccount.Connector {
	filePath := credscanning.LoadPath(providers.CyclrAccount)
	reader := utils.MustCreateProvCredJSON(filePath, false, fieldAPIDomain, fieldAccountApiId)

	apiDomain := reader.Get(fieldAPIDomain)
	if apiDomain == "" {
		utils.Fail("cyclrAccount creds missing metadata.apiDomain")
	}

	accountApiId := reader.Get(fieldAccountApiId)
	if accountApiId == "" {
		utils.Fail("cyclrAccount creds missing metadata.accountApiId")
	}

	cfg := &clientcredentials.Config{
		ClientID:     reader.Get(credscanning.Fields.ClientId),
		ClientSecret: reader.Get(credscanning.Fields.ClientSecret),
		TokenURL:     fmt.Sprintf("https://%s/oauth/token", apiDomain),
		Scopes:       []string{fmt.Sprintf("account:%s", accountApiId)},
		AuthStyle:    oauth2.AuthStyleInParams,
	}

	conn, err := cyclraccount.NewConnector(common.ConnectorParams{
		AuthenticatedClient: oauth2.NewClient(ctx, cfg.TokenSource(ctx)),
		Metadata: map[string]string{
			"apiDomain":    apiDomain,
			"accountApiId": accountApiId,
		},
	})
	if err != nil {
		utils.Fail("error creating connector", "error", err)
	}

	return conn
}
