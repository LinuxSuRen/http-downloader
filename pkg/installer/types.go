package installer

// HDConfig is the config of http-downloader
type HDConfig struct {
	Name         string            `yaml:"Name"`
	Filename     string            `yaml:"filename"`
	Binary       string            `yaml:"binary"`
	TargetBinary string            `yaml:"targetBinary"`
	URL          string            `yaml:"url"`
	Tar          string            `yaml:"tar"`
	SupportOS    []string          `yaml:"supportOS"`
	SupportArch  []string          `yaml:"supportArch"`
	Replacements map[string]string `yaml:"replacements"`
	Installation *CmdWithArgs      `yaml:"installation"`
	PreInstall   *CmdWithArgs      `yaml:"preInstall"`
	PostInstall  *CmdWithArgs      `yaml:"postInstall"`
	TestInstall  *CmdWithArgs      `yaml:"testInstall"`
}

// CmdWithArgs is a command with arguments
type CmdWithArgs struct {
	Cmd  string   `yaml:"cmd"`
	Args []string `yaml:"args"`
}

// HDPackage represents a package of http-downloader
type HDPackage struct {
	Name       string
	Version    string // e.g. v1.0.1
	VersionNum string // e.g. 1.0.1
	OS         string // e.g. linux, darwin
	Arch       string // e.g. amd64
}

// Installer is a tool to install a package
type Installer struct {
	Package      *HDConfig
	Tar          bool
	Output       string
	Source       string
	Name         string
	CleanPackage bool
	Provider     string
	OS           string
	Arch         string
	Fetch        bool
}
