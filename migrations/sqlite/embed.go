// Package sqlitemigrations provides embedded SQLite migration files.
package sqlitemigrations

import "embed"

//go:embed *.sql
var FS embed.FS
