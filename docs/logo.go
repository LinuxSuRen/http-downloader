// Package docs keeps the project branding assets. The files next to this
// one are the single source of the logo: the README and the web console
// reference them, no copy lives anywhere else.
package docs

import (
	// embed is required by the go:embed directive below, it pulls the
	// logo file into the binary
	_ "embed"
)

// LogoSVG is the project logo, the web console serves it as the favicon
// and the header brand.
//
//go:embed logo.svg
var LogoSVG []byte
