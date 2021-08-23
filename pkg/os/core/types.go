package core

// Installer is the interface of a installer
type Installer interface {
	Available() bool
	Install() error
	Uninstall() error

	WaitForStart() (bool, error)
	Start() error
	Stop() error
}

// InstallerRegistry is the interface of install registry
type InstallerRegistry interface {
	Registry(string, Installer)
}
