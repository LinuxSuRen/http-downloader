package os

type genericInstaller struct {
}

type genericPackage struct {
	Alias string
	Name           string
	OS             string
	PackageManager string
	PreInstall string
	Dependents []string
	Install        string
	Uninstall      string
	IsService      bool
	Start          string
	Stop           string
}

// CmdWithArgs is a command with arguments
type CmdWithArgs struct {
	Cmd  string   `yaml:"cmd"`
	Args []string `yaml:"args"`
}

func NewGenericInstaller(configFile string) (installer *os.AdvanceInstaller) {
	installer = &genericInstaller{}
	return
}

func (i *genericInstaller) Available() bool {
	return false
}
func (i *genericInstaller) Install() error {
	return nil
}
func (i *genericInstaller) Uninstall() error {
	return nil
}

func (i *genericInstaller) IsService() bool {
	return false
}
func (i *genericInstaller) WaitForStart() (bool, error) {
	return false, nil
}
func (i *genericInstaller) Start() error {
	return nil
}
func (i *genericInstaller) Stop() error {
	return nil
}
