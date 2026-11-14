// Package docs carries the brand assets of the project. The logo has a
// single source file here, the README and the web console embed the very
// same bytes.
package docs

import _ "embed"

// Logo is the SVG logo of the project
//
//go:embed logo.svg
var Logo []byte
