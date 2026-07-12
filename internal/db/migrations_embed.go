package db

import "embed"

// migrationsFS embeds the goose SQL migrations into the binary so migrations
// run identically in dev, CI, and production without shipping loose files.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS
