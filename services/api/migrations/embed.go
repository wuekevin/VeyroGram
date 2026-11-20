// Package migrations embeds the SQL migrations (forward-only versions, each with a down script). Run them with `veyrogram-api migrate up`.
package migrations

import "embed"

// FS holds NNNN_name.up.sql / NNNN_name.down.sql files.
//
//go:embed *.sql
var FS embed.FS
