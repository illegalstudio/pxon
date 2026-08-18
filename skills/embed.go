// Package skills provides the AI agent skill bundled with pxon.
package skills

import "embed"

// bundled contains the skill exactly as it is installed for the user.
//
//go:embed pxon
var bundled embed.FS
