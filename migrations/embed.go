// Package migrations embeds the SQL migrations so a single binary can
// bootstrap a fresh database (docker compose up from zero) before the HTTP
// server starts listening.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
