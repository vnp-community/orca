// Package schemas embeds the versioned JSON Schemas of the artifact model (CR-REQ-027).
// It is separate from internal/domain so the domain keeps receiving the files as an fs.FS.
package schemas

import "embed"

//go:embed v1/*.schema.json
var V1 embed.FS
