package installer

import (
	"runtime"
	"strings"
)

const (
	// UnixBinDir is the conventional binary directory on Linux and macOS
	UnixBinDir = "/usr/local/bin"
	// WindowsBinDir is a user writable directory which is on PATH by
	// default. Never join a Unix path on Windows: filepath.Join turns
	// /usr/local/bin into C:\usr\local\bin.
	WindowsBinDir = `C:\Program Files (x86)\Common Files`
)

// DefaultInstallDir returns the default installation directory of the given
// platform. An empty result means the platform is unknown, callers should
// then fall back to the directory of the machine hd is running on.
func DefaultInstallDir(goos string) string {
	switch strings.ToLower(goos) {
	case "linux", "darwin":
		return UnixBinDir
	case "windows":
		return WindowsBinDir
	}
	return ""
}

// ResolveInstallDir picks the installation directory. An explicitly
// configured value always wins, then the package specific value, then the
// platform default of the target system and finally the platform of the
// machine hd is running on.
func ResolveInstallDir(configured, packageDir, targetOS string) string {
	for _, candidate := range []string{configured, packageDir, DefaultInstallDir(targetOS)} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return DefaultInstallDir(runtime.GOOS)
}
