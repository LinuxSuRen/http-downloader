// Package native provides the installation of the OS level installer
// packages, for instance: deb, rpm, msi and pkg. It reuses the package
// tools which are already shipped by the operating system, for instance:
// apt-get, dnf and msiexec.
package native

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

// Format represents a native installer package format
type Format string

const (
	// FormatDeb is the package format of Debian, Ubuntu and their derivatives
	FormatDeb Format = "deb"
	// FormatRPM is the package format of RHEL, Fedora, CentOS, openSUSE etc
	FormatRPM Format = "rpm"
	// FormatMSI is the Windows Installer package format
	FormatMSI Format = "msi"
	// FormatPkg is the installer package format of macOS
	FormatPkg Format = "pkg"
	// FormatApk is the package format of Alpine Linux
	FormatApk Format = "apk"
	// FormatPacman is the package format of Arch Linux, the file name
	// usually looks like xxx.pkg.tar.zst or xxx.pkg.tar.xz
	FormatPacman Format = "pacman"
)

var pacmanSuffixes = []string{".pkg.tar.zst", ".pkg.tar.xz", ".pkg.tar.gz"}

// Detect returns the native installer package format of the given file.
// An empty format means the file is not a native installer package.
func Detect(name string) (format Format) {
	lower := strings.ToLower(name)

	// the pacman packages must be checked before the macOS .pkg packages
	// because their file names end with .zst or .xz instead of .pkg
	for _, suffix := range pacmanSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return FormatPacman
		}
	}

	switch filepath.Ext(lower) {
	case ".deb":
		return FormatDeb
	case ".rpm":
		return FormatRPM
	case ".msi":
		return FormatMSI
	case ".pkg":
		return FormatPkg
	case ".apk":
		return FormatApk
	}
	return ""
}

// IsInstallerFile checks if the given file looks like a native installer
// package which can be installed by the system package tools
func IsInstallerFile(name string) bool {
	return Detect(name) != ""
}

// Installer installs native installer packages via the system package tools
type Installer struct {
	// Execer is used to run the system package tools
	Execer fakeruntime.Execer

	// Root indicates the current process already has the privileges which
	// the system package tools need. It's only meaningful on Linux and macOS.
	// Leave it as the zero value to detect automatically.
	Root bool

	// inner fields
	euid func() int
}

// Install installs the given native installer package file
func (i *Installer) Install(file string) (err error) {
	format := Detect(file)
	if format == "" {
		return fmt.Errorf("not a supported native installer package: %s", file)
	}

	var absFile string
	if absFile, err = filepath.Abs(file); err != nil {
		return fmt.Errorf("cannot get the absolute path of %s, error: %v", file, err)
	}

	if i.Execer == nil {
		i.Execer = fakeruntime.NewDefaultExecer()
	}

	switch format {
	case FormatDeb:
		return i.installDeb(absFile)
	case FormatRPM:
		return i.installRPM(absFile)
	case FormatMSI:
		return i.installMSI(absFile)
	case FormatPkg:
		return i.installPkg(absFile)
	case FormatApk:
		return i.installApk(absFile)
	case FormatPacman:
		return i.installPacman(absFile)
	}
	return fmt.Errorf("unsupported native installer package format: %s", format)
}

// installDeb prefers apt-get which resolves the dependencies automatically,
// then falls back to dpkg
func (i *Installer) installDeb(file string) (err error) {
	if _, lookErr := i.Execer.LookPath("apt-get"); lookErr == nil {
		return i.run("apt-get", "install", "-y", file)
	}
	if _, lookErr := i.Execer.LookPath("dpkg"); lookErr == nil {
		return i.run("dpkg", "-i", file)
	}
	return fmt.Errorf("cannot install %s, neither apt-get nor dpkg was found", file)
}

// installRPM prefers the high level package managers which resolve the
// dependencies automatically, then falls back to the low level rpm command
func (i *Installer) installRPM(file string) (err error) {
	if _, lookErr := i.Execer.LookPath("dnf"); lookErr == nil {
		return i.run("dnf", "install", "-y", file)
	}
	if _, lookErr := i.Execer.LookPath("yum"); lookErr == nil {
		return i.run("yum", "localinstall", "-y", file)
	}
	if _, lookErr := i.Execer.LookPath("rpm"); lookErr == nil {
		return i.run("rpm", "-i", "--replacepkgs", file)
	}
	return fmt.Errorf("cannot install %s, none of dnf, yum and rpm was found", file)
}

// installMSI installs a Windows Installer package. msiexec always runs in
// the current context, an elevated terminal might be needed.
func (i *Installer) installMSI(file string) (err error) {
	if i.Execer.OS() != fakeruntime.OSWindows {
		return fmt.Errorf("msi packages are only supported on Windows")
	}
	return i.Execer.RunCommand("msiexec", "/i", file, "/passive", "/norestart")
}

// installPkg installs a macOS installer package to the root volume
func (i *Installer) installPkg(file string) (err error) {
	if i.Execer.OS() != fakeruntime.OSDarwin {
		return fmt.Errorf("pkg packages are only supported on macOS")
	}
	return i.run("installer", "-pkg", file, "-target", "/")
}

// installApk installs an Alpine Linux package file. The local file is not
// managed by a trusted key, so --allow-untrusted is required.
func (i *Installer) installApk(file string) (err error) {
	return i.run("apk", "add", "--allow-untrusted", file)
}

// installPacman installs an Arch Linux package file
func (i *Installer) installPacman(file string) (err error) {
	return i.run("pacman", "-U", "--noconfirm", file)
}

// run runs a package tool, with sudo when the privileges are missing
func (i *Installer) run(name string, args ...string) (err error) {
	if i.needSudo() {
		if _, lookErr := i.Execer.LookPath("sudo"); lookErr == nil {
			return i.Execer.RunCommandWithSudo(name, args...)
		}
	}
	return i.Execer.RunCommand(name, args...)
}

// needSudo checks if the package tools should be run via sudo
func (i *Installer) needSudo() bool {
	if i.Root || i.Execer.OS() == fakeruntime.OSWindows {
		return false
	}
	if i.euid == nil {
		i.euid = os.Geteuid
	}
	return i.euid() != 0
}
