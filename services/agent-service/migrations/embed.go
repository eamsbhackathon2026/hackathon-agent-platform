// Package migrations embeds database migrations in the service binary.
package migrations

import "embed"

// Files includes SQL migrations when present. Goose ignores non-SQL files.
// Embedding the directory also permits a fresh project without SQL migrations.
//
//go:embed *
var Files embed.FS
