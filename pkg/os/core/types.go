package core

type Installer interface {
	Available() bool
	Install() error
	Uninstall() error
}

type InstallerRegistry interface {
	Registry(string, Installer)
}
