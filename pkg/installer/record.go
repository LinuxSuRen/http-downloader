package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v3"
)

// InstalledPackage represents a tool which is installed by hd
type InstalledPackage struct {
	Name            string   `yaml:"name" json:"name"`
	Org             string   `yaml:"org,omitempty" json:"org,omitempty"`
	Repo            string   `yaml:"repo,omitempty" json:"repo,omitempty"`
	Version         string   `yaml:"version,omitempty" json:"version,omitempty"`
	Binaries        []string `yaml:"binaries,omitempty" json:"binaries,omitempty"`
	TargetDirectory string   `yaml:"targetDirectory,omitempty" json:"targetDirectory,omitempty"`
	InstalledAt     string   `yaml:"installedAt,omitempty" json:"installedAt,omitempty"`
	FromSource      bool     `yaml:"fromSource,omitempty" json:"fromSource,omitempty"`
	Native          bool     `yaml:"native,omitempty" json:"native,omitempty"`
}

// installedFile is the structure of the installed tools record file
type installedFile struct {
	Tools []InstalledPackage `yaml:"tools"`
}

// GetInstalledFile returns the path of the installed tools record file
func GetInstalledFile() (path string, err error) {
	var userHome string
	if userHome, err = homedir.Dir(); err != nil {
		return
	}
	path = filepath.Join(userHome, ".config", "hd-installed.yaml")
	return
}

// LoadInstalledPackages loads all the installed tools from the record file
func LoadInstalledPackages() (tools []InstalledPackage, err error) {
	var path string
	if path, err = GetInstalledFile(); err != nil {
		return
	}

	var data []byte
	if data, err = os.ReadFile(path); err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}

	file := &installedFile{}
	if err = yaml.Unmarshal(data, file); err != nil {
		err = fmt.Errorf("cannot parse the installed tools file: %s, error: %v", path, err)
		return
	}
	tools = file.Tools
	return
}

// SaveInstalledPackages saves the installed tools into the record file
func SaveInstalledPackages(tools []InstalledPackage) (err error) {
	var path string
	if path, err = GetInstalledFile(); err != nil {
		return
	}

	if err = os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return
	}

	var data []byte
	if data, err = yaml.Marshal(&installedFile{Tools: tools}); err != nil {
		return
	}
	err = os.WriteFile(path, data, 0644)
	return
}

// FindInstalledPackage finds an installed tool by its name
func FindInstalledPackage(name string) (target *InstalledPackage, index int) {
	tools, err := LoadInstalledPackages()
	if err != nil {
		return nil, -1
	}
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i], i
		}
	}
	return nil, -1
}

// RecordInstalledPackage adds or updates a tool in the record file
func RecordInstalledPackage(tool InstalledPackage) (err error) {
	tools, err := LoadInstalledPackages()
	if err != nil {
		return
	}

	found := false
	for i := range tools {
		if tools[i].Name == tool.Name {
			// keep the original install time if it exists
			if tool.InstalledAt == "" {
				tool.InstalledAt = tools[i].InstalledAt
			}
			tools[i] = tool
			found = true
			break
		}
	}
	if !found {
		tools = append(tools, tool)
	}
	return SaveInstalledPackages(tools)
}

// RemoveInstalledPackage removes a tool from the record file by its name
func RemoveInstalledPackage(name string) (removed *InstalledPackage, err error) {
	tools, err := LoadInstalledPackages()
	if err != nil {
		return
	}

	index := -1
	for i := range tools {
		if tools[i].Name == name {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("cannot find the installed tool: %s", name)
	}

	removed = &tools[index]
	tools = append(tools[:index], tools[index+1:]...)
	err = SaveInstalledPackages(tools)
	return
}
