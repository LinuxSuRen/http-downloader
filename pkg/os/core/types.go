package core

// Installer is the interface of a installer
// Deprecated use AdvanceInstaller instead
type Installer interface {
	Available() bool
	Install() error
	Uninstall() error

	WaitForStart() (bool, error)
	Start() error
	Stop() error
}

// AdvanceInstaller is a generic installer
type AdvanceInstaller interface {
	Installer

	IsService() bool
}

// InstallerRegistry is the interface of install registry
type InstallerRegistry interface {
	Registry(string, Installer)
}

// ProxyAble define the proxy support feature
type ProxyAble interface {
	SetURLReplace(map[string]string)
}
