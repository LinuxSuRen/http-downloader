package installer

// PackagingFormat is used for containing config depending on machine
type PackagingFormat struct {
	Windows string `yaml:"windows"`
	Linux   string `yaml:"linux"`
}

// HDConfig is the config of http-downloader
type HDConfig struct {
	Name             string            `yaml:"name"`
	Categories       []string          `yaml:"categories"`
	Filename         string            `yaml:"filename"`
	FormatOverrides  PackagingFormat   `yaml:"formatOverrides"`
	Binary           string            `yaml:"binary"`
	TargetBinary     string            `yaml:"targetBinary"`
	AdditionBinaries []string          `yaml:"additionBinaries"`
	FromSource       bool              `yaml:"fromSource"`
	URL              string            `yaml:"url"`
	Tar              string            `yaml:"tar"`
	LatestVersion    string            `yaml:"latestVersion"`
	SupportOS        []string          `yaml:"supportOS"`
	SupportArch      []string          `yaml:"supportArch"`
	Replacements     map[string]string `yaml:"replacements"`
	Requirements     []string          `yaml:"requirements"`
	Installation     *CmdWithArgs      `yaml:"installation"`
	PreInstalls      []CmdWithArgs     `yaml:"preInstalls"`
	PostInstalls     []CmdWithArgs     `yaml:"postInstalls"`
	TestInstalls     []CmdWithArgs     `yaml:"testInstalls"`

	Org, Repo string
}

// CmdWithArgs is a command with arguments
type CmdWithArgs struct {
	Cmd  string   `yaml:"cmd"`
	Args []string `yaml:"args"`
}

// HDPackage represents a package of http-downloader
type HDPackage struct {
	Name             string
	Version          string // e.g. v1.0.1
	VersionNum       string // e.g. 1.0.1
	OS               string // e.g. linux, darwin
	Arch             string // e.g. amd64
	AdditionBinaries []string
}

// Installer is a tool to install a package
type Installer struct {
	Package          *HDConfig
	Tar              bool
	Output           string
	Source           string
	Name             string
	CleanPackage     bool
	Provider         string
	OS               string
	Arch             string
	Fetch            bool
	AdditionBinaries []string

	Org  string
	Repo string
}
