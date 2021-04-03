package installer

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

type CmdWithArgs struct {
	Cmd  string   `yaml:"cmd"`
	Args []string `yaml:"args"`
}

type HDPackage struct {
	Name       string
	Version    string // e.g. v1.0.1
	VersionNum string // e.g. 1.0.1
	OS         string // e.g. linux, darwin
	Arch       string // e.g. amd64
}

type Installer struct {
	Package      *HDConfig
	Tar          bool
	Output       string
	Source       string
	Name         string
	CleanPackage bool
}
