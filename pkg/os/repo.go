package os

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/linuxsuren/http-downloader/pkg/os/apk"
	"github.com/linuxsuren/http-downloader/pkg/os/apt"
	"github.com/linuxsuren/http-downloader/pkg/os/brew"
	"github.com/linuxsuren/http-downloader/pkg/os/choco"
	"github.com/linuxsuren/http-downloader/pkg/os/core"
	"github.com/linuxsuren/http-downloader/pkg/os/dnf"
	"github.com/linuxsuren/http-downloader/pkg/os/flatpak"
	"github.com/linuxsuren/http-downloader/pkg/os/npm"
	"github.com/linuxsuren/http-downloader/pkg/os/pacman"
	"github.com/linuxsuren/http-downloader/pkg/os/scoop"
	"github.com/linuxsuren/http-downloader/pkg/os/snap"
	"github.com/linuxsuren/http-downloader/pkg/os/winget"
	"github.com/linuxsuren/http-downloader/pkg/os/yum"
	"github.com/linuxsuren/http-downloader/pkg/os/zypper"
)

// The names of the supported system package managers. They are also the
// prefixes of the package references, for instance: apt:jenkins
const (
	ManagerApk     = "apk"
	ManagerApt     = "apt"
	ManagerBrew    = "brew"
	ManagerChoco   = "choco"
	ManagerDnf     = "dnf"
	ManagerFlatpak = "flatpak"
	ManagerNpm     = "npm"
	ManagerPacman  = "pacman"
	ManagerScoop   = "scoop"
	ManagerSnap    = "snap"
	ManagerWinget  = "winget"
	ManagerYum     = "yum"
	ManagerZypper  = "zypper"
)

// DefaultSearchTimeout is the default timeout of a single package manager
// search command, a slow manager never blocks the whole search longer
var DefaultSearchTimeout = 15 * time.Second

// maxResultsPerManager limits the results of a single package manager, so
// that a broad keyword cannot flood the output
const maxResultsPerManager = 30

// SupportedManagers returns the names of the supported package managers
func SupportedManagers() []string {
	return []string{
		ManagerApt, ManagerDnf, ManagerYum, ManagerApk, ManagerPacman, ManagerZypper,
		ManagerSnap, ManagerFlatpak, ManagerBrew,
		ManagerWinget, ManagerChoco, ManagerScoop, ManagerNpm,
	}
}

// ParseManagerRef parses a package reference which carries a package
// manager prefix, for instance: apt:jenkins, winget:Git.Git. The second
// return value is the package manager, the third one is the package name.
func ParseManagerRef(ref string) (manager, name string, ok bool) {
	idx := strings.Index(ref, ":")
	if idx <= 0 || idx == len(ref)-1 {
		return
	}

	manager = ref[:idx]
	name = ref[idx+1:]
	for _, item := range SupportedManagers() {
		if item == manager {
			ok = true
			break
		}
	}
	return
}

// NewManagerInstaller creates an installer which installs or uninstalls a
// package via the given system package manager. All the installers reuse
// the package tools which are already shipped by the operating system.
func NewManagerInstaller(manager, name string, execer fakeruntime.Execer) (installer core.Installer, err error) {
	if execer == nil {
		execer = fakeruntime.NewDefaultExecer()
	}

	switch manager {
	case ManagerApk:
		installer = &apk.CommonInstaller{Name: name, Execer: execer}
	case ManagerApt:
		installer = &apt.CommonInstaller{Name: name, Execer: execer}
	case ManagerBrew:
		installer = &brew.CommonInstaller{Name: name, Execer: execer}
	case ManagerChoco:
		installer = &choco.CommonInstaller{Name: name, Execer: execer}
	case ManagerDnf:
		installer = &dnf.CommonInstaller{Name: name, Execer: execer}
	case ManagerFlatpak:
		installer = &flatpak.CommonInstaller{Name: name, Execer: execer}
	case ManagerNpm:
		installer = &npm.CommonInstaller{Name: name, Execer: execer}
	case ManagerPacman:
		installer = &pacman.CommonInstaller{Name: name, Execer: execer}
	case ManagerScoop:
		installer = &scoop.CommonInstaller{Name: name, Execer: execer}
	case ManagerSnap:
		installer = &snap.CommonInstaller{Name: name, Execer: execer}
	case ManagerWinget:
		installer = &winget.CommonInstaller{Name: name, Execer: execer}
	case ManagerYum:
		installer = &yum.CommonInstaller{Name: name, Execer: execer}
	case ManagerZypper:
		installer = &zypper.CommonInstaller{Name: name, Execer: execer}
	default:
		err = fmt.Errorf("unsupported package manager: %s", manager)
	}
	return
}

// InstallWithManager installs a package via the given system package manager,
// a nil execer falls back to the default one
func InstallWithManager(manager, name string, execer fakeruntime.Execer) (err error) {
	installer, err := NewManagerInstaller(manager, name, execer)
	if err != nil {
		return
	}
	if !installer.Available() {
		return fmt.Errorf("the package manager %s is not available on this platform", manager)
	}
	return installer.Install()
}

// UninstallWithManager uninstalls a package via the given system package
// manager, a nil execer falls back to the default one
func UninstallWithManager(manager, name string, execer fakeruntime.Execer) (err error) {
	installer, err := NewManagerInstaller(manager, name, execer)
	if err != nil {
		return
	}
	if !installer.Available() {
		return fmt.Errorf("the package manager %s is not available on this platform", manager)
	}
	return installer.Uninstall()
}

// RepoPackage is a package which comes from a system package repository,
// for instance: the jenkins package of the Ubuntu repositories
type RepoPackage struct {
	Manager     string
	Name        string
	Version     string
	Description string
}

// Ref returns the installable reference of the package, for instance:
// apt:jenkins. It can be consumed by the install command directly.
func (p RepoPackage) Ref() string {
	return p.Manager + ":" + p.Name
}

// commandRunner runs a command and returns its combined output, it is
// overridable for the test purposes
type commandRunner func(name string, args ...string) ([]byte, error)

// RepositorySearcher searches packages from the system package repositories.
// It reuses the search commands of the package managers which are already
// installed, no repository metadata is parsed or duplicated here.
type RepositorySearcher struct {
	// Execer decides which package managers are available
	Execer fakeruntime.Execer
	// Timeout is the per manager timeout, DefaultSearchTimeout is used
	// when it is zero
	Timeout time.Duration
	// RunCommand executes the search commands, it is overridable in tests
	RunCommand commandRunner
}

// Search searches the keyword across the given package repositories. An
// empty managers slice means all the available ones. It never fails: the
// managers which are not installed or which error out are skipped.
func (s *RepositorySearcher) Search(keyword string, managers []string) (result []RepoPackage) {
	if s.Execer == nil {
		s.Execer = fakeruntime.NewDefaultExecer()
	}
	if s.Timeout <= 0 {
		s.Timeout = DefaultSearchTimeout
	}
	if s.RunCommand == nil {
		timeout := s.Timeout
		s.RunCommand = func(name string, args ...string) ([]byte, error) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			// #nosec G204 -- the commands come from the built-in manager
			// specs, not from the user input
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		}
	}

	managerFilter := make(map[string]bool, len(managers))
	for i := range managers {
		managerFilter[strings.TrimSpace(managers[i])] = true
	}

	results := make([][]RepoPackage, len(managerSpecs))
	var wait sync.WaitGroup
	for i := range managerSpecs {
		spec := managerSpecs[i]
		if len(managerFilter) > 0 {
			// an explicit filter always wins, so the opt-out managers are
			// still searchable on demand
			if !managerFilter[spec.name] {
				continue
			}
		} else if !spec.defaultSearch {
			continue
		}
		if !spec.available(s.Execer) {
			continue
		}

		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			args := append([]string{}, spec.searchArgs(keyword)...)
			if output, err := s.RunCommand(spec.tool, args...); err == nil {
				results[index] = spec.parse(string(output))
			}
		}(i)
	}
	wait.Wait()

	// keep the results in the manager order, so the output is stable
	for i := range results {
		result = append(result, results[i]...)
	}
	return
}

// managerSpec describes how to search packages against one package manager
type managerSpec struct {
	name string
	oses []string
	tool string
	// defaultSearch tells whether the manager takes part in the default
	// repository search, the noisy ones can be requested explicitly
	defaultSearch bool
	// searchArgs builds the search command arguments of the keyword
	searchArgs func(keyword string) []string
	// parse extracts the packages out of the search output, best-effort
	parse func(output string) []RepoPackage
}

func (m *managerSpec) available(execer fakeruntime.Execer) bool {
	osMatched := len(m.oses) == 0
	for _, item := range m.oses {
		if execer.OS() == item {
			osMatched = true
			break
		}
	}
	if !osMatched {
		return false
	}
	_, err := execer.LookPath(m.tool)
	return err == nil
}

// spacesPattern splits the table style output of the package managers
var spacesPattern = regexp.MustCompile(`\s{2,}`)

// apkNamePattern cuts the version suffix of an apk package, for instance:
// jenkins-2.426-1 becomes jenkins
var apkNamePattern = regexp.MustCompile(`^(.+?)-[0-9]`)

// trimPackages trims, drops the empty lines and limits the result count
func trimPackages(packages []RepoPackage, manager string) (result []RepoPackage) {
	for i := range packages {
		item := packages[i]
		item.Manager = manager
		item.Name = strings.TrimSpace(item.Name)
		item.Version = strings.TrimSpace(item.Version)
		item.Description = strings.TrimSpace(item.Description)
		if item.Name == "" || strings.ContainsAny(item.Name, " \t") {
			continue
		}
		result = append(result, item)
		if len(result) >= maxResultsPerManager {
			break
		}
	}
	return
}

// parseLinesOutput parses the "name - description" or "name: description"
// style output, for instance: apt-cache search, dnf search and yum search
func parseColonOutput(output, separator string, manager string) (result []RepoPackage) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "=") {
			// the section banners of dnf and yum
			continue
		}

		var pkg RepoPackage
		if idx := strings.Index(line, separator); idx > 0 {
			pkg.Name = line[:idx]
			pkg.Description = strings.TrimSpace(line[idx+len(separator):])
		} else {
			pkg.Name = line
		}
		result = append(result, pkg)
	}
	return trimPackages(result, manager)
}

// parseColumnsOutput parses the table style output which separates the
// columns with two or more spaces, for instance: winget, snap and flatpak
func parseColumnsOutput(output string, manager string, hasHeader func(columns []string) bool,
	toPackage func(columns []string) RepoPackage) (result []RepoPackage) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}

		columns := spacesPattern.Split(strings.TrimSpace(line), -1)
		if len(columns) == 0 || hasHeader(columns) {
			continue
		}

		if pkg := toPackage(columns); pkg.Name != "" {
			result = append(result, pkg)
		}
	}
	return trimPackages(result, manager)
}

// managerSpecs holds the search behavior of every supported package manager
var managerSpecs = []managerSpec{{
	name:          ManagerApt,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSLinux},
	tool:          "apt-cache",
	searchArgs: func(keyword string) []string {
		return []string{"search", "--names-only", keyword}
	},
	parse: func(output string) []RepoPackage {
		return parseColonOutput(output, " - ", ManagerApt)
	},
}, {
	name:          ManagerDnf,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSLinux},
	tool:          ManagerDnf,
	searchArgs: func(keyword string) []string {
		return []string{"--quiet", "search", keyword}
	},
	parse: func(output string) []RepoPackage {
		return parseColonOutput(output, ":", ManagerDnf)
	},
}, {
	name:          ManagerYum,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSLinux},
	tool:          ManagerYum,
	searchArgs: func(keyword string) []string {
		return []string{"--quiet", "search", keyword}
	},
	parse: func(output string) []RepoPackage {
		return parseColonOutput(output, ":", ManagerYum)
	},
}, {
	name:          ManagerApk,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSLinux},
	tool:          ManagerApk,
	searchArgs: func(keyword string) []string {
		return []string{"search", keyword}
	},
	parse: func(output string) (result []RepoPackage) {
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			var pkg RepoPackage
			if idx := strings.IndexByte(line, ' '); idx > 0 {
				pkg.Name = line[:idx]
				pkg.Description = line[idx+1:]
			} else {
				pkg.Name = line
			}

			// the package token carries the version, for instance:
			// jenkins-2.426-1
			if found := apkNamePattern.FindStringSubmatch(pkg.Name); found != nil {
				pkg.Name = found[1]
			}
			result = append(result, pkg)
		}
		return trimPackages(result, ManagerApk)
	},
}, {
	name:          ManagerPacman,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSLinux},
	tool:          ManagerPacman,
	searchArgs: func(keyword string) []string {
		return []string{"-Ss", keyword}
	},
	parse: func(output string) (result []RepoPackage) {
		var last RepoPackage
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimRight(line, "\r")
			if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ") {
				// the description line of the previous package
				last.Description = strings.TrimSpace(line)
				if last.Name != "" {
					result = append(result, last)
				}
				last = RepoPackage{}
				continue
			}

			columns := strings.SplitN(strings.TrimSpace(line), " ", 2)
			if len(columns) < 2 {
				continue
			}
			// the first column looks like core/bash, only the package
			// name is installable
			name := columns[0]
			if idx := strings.IndexByte(name, '/'); idx >= 0 {
				name = name[idx+1:]
			}
			last = RepoPackage{Name: name, Version: columns[1]}
		}
		return trimPackages(result, ManagerPacman)
	},
}, {
	name:          ManagerZypper,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSLinux},
	tool:          ManagerZypper,
	searchArgs: func(keyword string) []string {
		return []string{"--non-interactive", "search", keyword}
	},
	parse: func(output string) (result []RepoPackage) {
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimRight(line, "\r")
			if strings.Trim(line, "-+ ") == "" {
				// the table separator line
				continue
			}
			columns := strings.Split(line, "|")
			if len(columns) < 4 {
				continue
			}

			pkg := RepoPackage{
				Name:    strings.TrimSpace(columns[1]),
				Version: strings.TrimSpace(columns[3]),
			}
			if pkg.Name == "Name" || pkg.Name == "" {
				continue
			}
			result = append(result, pkg)
		}
		return trimPackages(result, ManagerZypper)
	},
}, {
	name:          ManagerSnap,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSLinux},
	tool:          ManagerSnap,
	searchArgs: func(keyword string) []string {
		return []string{"find", keyword}
	},
	parse: func(output string) []RepoPackage {
		return parseColumnsOutput(output, ManagerSnap,
			func(columns []string) bool { return columns[0] == "Name" },
			func(columns []string) RepoPackage {
				pkg := RepoPackage{Name: columns[0]}
				if len(columns) > 1 {
					pkg.Version = columns[1]
				}
				if len(columns) > 4 {
					pkg.Description = columns[4]
				}
				return pkg
			})
	},
}, {
	name:          ManagerFlatpak,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSLinux},
	tool:          ManagerFlatpak,
	searchArgs: func(keyword string) []string {
		return []string{"search", keyword}
	},
	parse: func(output string) []RepoPackage {
		return parseColumnsOutput(output, ManagerFlatpak,
			func(columns []string) bool {
				// the header carries "Application ID" as one column
				return len(columns) > 1 && (columns[0] == "Name" || columns[1] == "Application ID")
			},
			func(columns []string) RepoPackage {
				if len(columns) < 3 {
					return RepoPackage{}
				}
				// the application ID is the installable identity
				pkg := RepoPackage{Name: columns[1], Description: columns[0], Version: columns[2]}
				return pkg
			})
	},
}, {
	name:          ManagerBrew,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSDarwin, fakeruntime.OSLinux},
	tool:          ManagerBrew,
	searchArgs: func(keyword string) []string {
		return []string{"search", keyword}
	},
	parse: func(output string) (result []RepoPackage) {
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "==>") {
				// the formulae and casks section headers
				continue
			}
			result = append(result, RepoPackage{Name: line})
		}
		return trimPackages(result, ManagerBrew)
	},
}, {
	name:          ManagerWinget,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSWindows},
	tool:          ManagerWinget,
	searchArgs: func(keyword string) []string {
		return []string{"search", "--accept-source-agreements", keyword}
	},
	parse: func(output string) []RepoPackage {
		return parseColumnsOutput(output, ManagerWinget,
			func(columns []string) bool {
				return columns[0] == "Name" || (len(columns) > 1 && columns[1] == "Id")
			},
			func(columns []string) RepoPackage {
				if len(columns) < 3 {
					return RepoPackage{}
				}
				// the Id column is the installable identity, the human
				// friendly name becomes the description
				return RepoPackage{Name: columns[1], Description: columns[0], Version: columns[2]}
			})
	},
}, {
	name:          ManagerChoco,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSWindows},
	tool:          ManagerChoco,
	searchArgs: func(keyword string) []string {
		return []string{"search", "--limit-output", keyword}
	},
	parse: func(output string) (result []RepoPackage) {
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			columns := strings.Split(line, "|")
			if len(columns) < 2 {
				continue
			}

			pkg := RepoPackage{Name: columns[0], Version: columns[1]}
			if len(columns) > 2 {
				pkg.Description = strings.Join(columns[2:], " ")
			}
			result = append(result, pkg)
		}
		return trimPackages(result, ManagerChoco)
	},
}, {
	name:          ManagerScoop,
	defaultSearch: true,
	oses:          []string{fakeruntime.OSWindows},
	tool:          ManagerScoop,
	searchArgs: func(keyword string) []string {
		return []string{"search", keyword}
	},
	parse: func(output string) []RepoPackage {
		return parseColumnsOutput(output, ManagerScoop,
			func(columns []string) bool { return columns[0] == "Name" },
			func(columns []string) RepoPackage {
				pkg := RepoPackage{Name: columns[0]}
				if len(columns) > 1 {
					pkg.Version = columns[1]
				}
				if len(columns) > 2 {
					pkg.Description = columns[2]
				}
				return pkg
			})
	},
}, {
	name: ManagerNpm,
	// the npm registry is noisy for tool oriented keywords, it takes part
	// in the search only when it is requested explicitly
	defaultSearch: false,
	tool:          ManagerNpm,
	searchArgs: func(keyword string) []string {
		return []string{"search", "--json", keyword}
	},
	parse: func(output string) (result []RepoPackage) {
		packages := make([]struct {
			Name        string
			Version     string
			Description string
		}, 0)
		if err := json.Unmarshal([]byte(output), &packages); err != nil {
			return
		}
		for i := range packages {
			result = append(result, RepoPackage{
				Name:        packages[i].Name,
				Version:     packages[i].Version,
				Description: packages[i].Description,
			})
		}
		return trimPackages(result, ManagerNpm)
	},
}}
