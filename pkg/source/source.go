// Package source contains the package registries which hd can search and
// install from. Keeping the source on a result makes it possible to tell a
// hd-home config apart from an npm package or an operating system package.
package source

const (
	// HDHome is the built-in config registry hosted at github.com/LinuxSuRen/hd-home
	HDHome = "hd-home"
	// NPM is the npm registry, packages are installed with npm install -g
	NPM = "npm"
	// Native is an operating system package manager such as apt, yum or brew
	Native = "native"
	// All matches every known source
	All = "all"
)

// Known returns true when the given source name is supported
func Known(name string) bool {
	switch name {
	case HDHome, NPM, Native, All:
		return true
	}
	return false
}

// Result is one package found in any of the supported sources
type Result struct {
	Source      string `json:"source"`
	Name        string `json:"name"`
	Org         string `json:"org,omitempty"`
	Repo        string `json:"repo,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
}

// ID returns the source specific package identifier. hd-home keeps the
// org/repo form so that the value can be passed directly to hd install.
func (r Result) ID() string {
	if r.Source == HDHome && r.Org != "" && r.Repo != "" {
		return r.Org + "/" + r.Repo
	}
	if r.Source == Native {
		return r.Name
	}
	return r.Source + ":" + r.Name
}

// String returns the identifier of the package
func (r Result) String() string {
	return r.ID()
}
