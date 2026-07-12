// Package migrations embeds the versioned SQL schema migrations so a
// single binary can carry and apply them without shipping loose files.
package migrations

import "embed"

// FS holds every migration file, applied in filename order by goose.
//
//go:embed *.sql
var FS embed.FS
