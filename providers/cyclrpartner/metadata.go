package cyclrpartner

import (
	_ "embed"

	"github.com/amp-labs/connectors/internal/staticschema"
	"github.com/amp-labs/connectors/tools/fileconv"
	"github.com/amp-labs/connectors/tools/scrapper"
)

// schemaContent embeds the hand-authored static schema file. Kept at package
// scope (and declared here rather than in connector.go) so tests and constructor
// share the same loaded instance.
//
//go:embed schemas.json
var schemaContent []byte

//nolint:gochecknoglobals
var (
	fileManager = scrapper.NewMetadataFileManager[staticschema.FieldMetadataMapV2](
		schemaContent, fileconv.NewSiblingFileLocator(),
	)

	// schemas is the loaded, immutable registry used by ListObjectMetadata.
	schemas = fileManager.MustLoadSchemas()
)
