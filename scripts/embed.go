package scripts

import "embed"

// Files contains the embedded installer scripts
//
//go:embed install-linux.sh install-windows.ps1 install-macos.sh
var Files embed.FS
