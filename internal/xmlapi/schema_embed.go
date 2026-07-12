package xmlapi

import (
	"embed"
	"io/fs"
)

// SchemaFS embeds the legacy XSD schemas so they can be served at /schema/<file>
// and used to validate generated XML.
//
//go:embed schema/*.xsd
var schemaFS embed.FS

// SchemaFiles returns the embedded XSD filesystem rooted at schema/.
func SchemaFiles() fs.FS {
	sub, _ := fs.Sub(schemaFS, "schema")
	return sub
}
