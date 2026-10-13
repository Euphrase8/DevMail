// Package migrations embeds the versioned SQL migration files that live in
// backend/migrations so the API can apply them without a separate runtime path.
package migrations

import "embed"

// FS holds every *.sql file in the backend/migrations directory.
//
//go:embed *.sql
var FS embed.FS