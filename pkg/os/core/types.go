package core

// Installer is the interface of a installer
type Installer interface {
	Available() bool
	Install() error
	Uninstall() error
}

// InstallerRegistry is the interface of install registry
type InstallerRegistry interface {
	Registry(string, Installer)
}
