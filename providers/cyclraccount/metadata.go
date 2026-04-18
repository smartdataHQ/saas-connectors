package cyclraccount

import (
	_ "embed"

	"github.com/amp-labs/connectors/internal/staticschema"
	"github.com/amp-labs/connectors/tools/fileconv"
	"github.com/amp-labs/connectors/tools/scrapper"
)

//go:embed schemas.json
var schemaContent []byte

//nolint:gochecknoglobals
var (
	fileManager = scrapper.NewMetadataFileManager[staticschema.FieldMetadataMapV2](
		schemaContent, fileconv.NewSiblingFileLocator(),
	)

	schemas = fileManager.MustLoadSchemas()
)
