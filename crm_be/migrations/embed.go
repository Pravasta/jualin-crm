// Package migrations embeds the SQL migration files so cmd/migrate can run
// them without the migrations/ directory needing to exist alongside the
// compiled binary (e.g. inside a minimal container image).
//
// The go:embed directive cannot reach outside its own package directory, which is why
// this tiny package lives next to the .sql files instead of embedding
// them directly from cmd/migrate.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
