package os

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/stretchr/testify/assert"
)

func TestParseManagerRef(t *testing.T) {
	for i, tt := range []struct {
		ref     string
		manager string
		name    string
		ok      bool
	}{{
		ref:     "apt:jenkins",
		manager: "apt",
		name:    "jenkins",
		ok:      true,
	}, {
		ref:     "winget:Git.Git",
		manager: "winget",
		name:    "Git.Git",
		ok:      true,
	}, {
		ref:     "npm:@angular/cli",
		manager: "npm",
		name:    "@angular/cli",
		ok:      true,
	}, {
		ref: "https://github.com",
		ok:  false,
	}, {
		ref: "apt:",
		ok:  false,
	}, {
		ref: ":jenkins",
		ok:  false,
	}, {
		ref: "unknown:jenkins",
		ok:  false,
	}, {
		ref: "jenkins",
		ok:  false,
	}, {
		ref: "linuxsuren/jenkins-cli",
		ok:  false,
	}} {
		t.Run(tt.ref, func(t *testing.T) {
			manager, name, ok := ParseManagerRef(tt.ref)
			assert.Equal(t, tt.ok, ok, "failed with case [%d]", i)
			if tt.ok {
				assert.Equal(t, tt.manager, manager)
				assert.Equal(t, tt.name, name)
			}
		})
	}
}

func TestNewManagerInstaller(t *testing.T) {
	for _, manager := range SupportedManagers() {
		installer, err := NewManagerInstaller(manager, "fake", &fakeruntime.FakeExecer{})
		assert.NoError(t, err, manager)
		assert.NotNil(t, installer, manager)
	}

	_, err := NewManagerInstaller("nope", "fake", &fakeruntime.FakeExecer{})
	assert.Error(t, err)

	// a nil execer falls back to the default one
	installer, err := NewManagerInstaller(ManagerApt, "fake", nil)
	assert.NoError(t, err)
	assert.NotNil(t, installer)
}

func TestInstallWithManager(t *testing.T) {
	// an unknown manager
	assert.Error(t, InstallWithManager("nope", "fake", nil))

	// winget is not available on Linux
	linuxExecer := &fakeruntime.FakeExecer{ExpectOS: "linux"}
	assert.Error(t, InstallWithManager(ManagerWinget, "fake", linuxExecer))
	assert.Error(t, UninstallWithManager(ManagerWinget, "fake", linuxExecer))

	// apt works on Linux with a fake execer
	assert.NoError(t, InstallWithManager(ManagerApt, "fake", linuxExecer))
	assert.NoError(t, UninstallWithManager(ManagerApt, "fake", linuxExecer))
}

func TestRepoPackageRef(t *testing.T) {
	pkg := RepoPackage{Manager: ManagerApt, Name: "jenkins"}
	assert.Equal(t, "apt:jenkins", pkg.Ref())
}

// searchOutputs carries the realistic search output samples of the package
// managers, they are keyed by the search tool name
var searchOutputs = map[string]string{
	"apt-cache": `jenkins - Jenkins continuous integration server
jenkins-job-builder - Configure Jenkins using YAML files
`,
	"dnf": `===================== Name Exactly Matched: jenkins =====================
jenkins : Jenkins continuous integration server
`,
	"yum": `jenkins : Jenkins automation server
`,
	"apk": `jenkins-2.426-r0 Jenkins automation server
`,
	"pacman": `extra/jenkins 2.426-1 [installed]
    Jenkins automation server
`,
	"zypper": `S | Name    | Type    | Version | Arch   | Repository
--+---------+---------+---------+--------+---------------------------
i | jenkins | package | 2.426-1 | x86_64 | openSUSE-Tumbleweed-Oss
`,
	"snap": `Name     Version  Publisher   Notes  Summary
jenkins  2.426    snapcraft✓  -      Jenkins automation server
`,
	"flatpak": `Name     Application ID          Version  Branch  Remotes
Jenkins  org.jenkins-ci.Jenkins  2.426    stable  flathub
`,
	"brew": `==> Formulae
jenkins
==> Casks
jenkins@lts
`,
	"winget": `Name     Id                       Version  Source
Jenkins  Jenkins.Project.Jenkins  2.426    winget
`,
	"choco": `jenkins|2.426|Jenkins automation server
`,
	"scoop": `Name     Version Source
jenkins  2.426   extras
`,
	"npm": `[{"name":"jenkins","version":"0.0.1","description":"node jenkins"}]
`,
}

func fakeSearcher(expectOS string) *RepositorySearcher {
	return &RepositorySearcher{
		Execer:  &fakeruntime.FakeExecer{ExpectOS: expectOS},
		Timeout: time.Second,
		RunCommand: func(name string, args ...string) ([]byte, error) {
			if output, ok := searchOutputs[name]; ok {
				return []byte(output), nil
			}
			return nil, fmt.Errorf("no fake output of %s", name)
		},
	}
}

func TestRepositorySearcherLinux(t *testing.T) {
	result := fakeSearcher("linux").Search("jenkins", nil)
	refs := make([]string, 0, len(result))
	for i := range result {
		refs = append(refs, result[i].Ref())
	}

	// every available Linux manager contributes one jenkins package
	assert.Contains(t, refs, "apt:jenkins")
	assert.Contains(t, refs, "dnf:jenkins")
	assert.Contains(t, refs, "yum:jenkins")
	assert.Contains(t, refs, "apk:jenkins")
	assert.Contains(t, refs, "pacman:jenkins")
	assert.Contains(t, refs, "zypper:jenkins")
	assert.Contains(t, refs, "snap:jenkins")
	assert.Contains(t, refs, "flatpak:org.jenkins-ci.Jenkins")
	assert.Contains(t, refs, "brew:jenkins")
	assert.Contains(t, refs, "brew:jenkins@lts")

	// npm opts out of the default search, it is too noisy for tool
	// oriented keywords, but an explicit filter includes it
	assert.NotContains(t, refs, "npm:jenkins")
	assert.Contains(t, fakeSearcher("linux").Search("jenkins", []string{ManagerNpm})[0].Ref(), "npm:jenkins")

	// the Windows only managers are filtered out
	for i := range result {
		assert.NotContains(t, []string{ManagerWinget, ManagerChoco, ManagerScoop}, result[i].Manager)
	}

	// the details of the parsed packages
	for i := range result {
		if result[i].Ref() == "apt:jenkins" {
			assert.Equal(t, "Jenkins continuous integration server", result[i].Description)
		}
		if result[i].Ref() == "zypper:jenkins" {
			assert.Equal(t, "2.426-1", result[i].Version)
		}
		if result[i].Ref() == "apk:jenkins" {
			assert.Equal(t, "Jenkins automation server", result[i].Description)
		}
	}

	// the results keep the manager order
	assert.True(t, strings.Index(strings.Join(refs, "\n"), "apt:jenkins") <
		strings.Index(strings.Join(refs, "\n"), "dnf:jenkins"))
}

func TestRepositorySearcherWindows(t *testing.T) {
	result := fakeSearcher("windows").Search("jenkins", nil)
	refs := make([]string, 0, len(result))
	for i := range result {
		refs = append(refs, result[i].Ref())
	}

	assert.Contains(t, refs, "winget:Jenkins.Project.Jenkins")
	assert.Contains(t, refs, "choco:jenkins")
	assert.Contains(t, refs, "scoop:jenkins")

	// npm only takes part in the search when it is requested explicitly
	assert.NotContains(t, refs, "npm:jenkins")
	assert.Contains(t, fakeSearcher("windows").Search("jenkins", []string{ManagerNpm})[0].Ref(), "npm:jenkins")

	// the Linux and macOS only managers are filtered out
	for i := range result {
		assert.NotContains(t, []string{ManagerApt, ManagerDnf, ManagerBrew}, result[i].Manager)
	}

	for i := range result {
		if result[i].Ref() == "winget:Jenkins.Project.Jenkins" {
			assert.Equal(t, "Jenkins", result[i].Description)
			assert.Equal(t, "2.426", result[i].Version)
		}
	}
}

func TestRepositorySearcherFilter(t *testing.T) {
	result := fakeSearcher("linux").Search("jenkins", []string{"apt", "dnf"})
	refs := make([]string, 0, len(result))
	for i := range result {
		refs = append(refs, result[i].Ref())
	}
	assert.Equal(t, []string{"apt:jenkins", "apt:jenkins-job-builder", "dnf:jenkins"}, refs)

	// an empty filter keeps the unknown managers away
	assert.Empty(t, fakeSearcher("linux").Search("jenkins", []string{"nope"}))
}

func TestRepositorySearcherToleratesFailures(t *testing.T) {
	searcher := &RepositorySearcher{
		Execer: &fakeruntime.FakeExecer{ExpectOS: "linux"},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			// only apt-cache works, all the other managers fail
			if name == "apt-cache" {
				return []byte(searchOutputs["apt-cache"]), nil
			}
			return nil, errors.New("search failed")
		},
	}
	result := searcher.Search("jenkins", nil)
	assert.Len(t, result, 2)
	assert.Equal(t, "apt:jenkins", result[0].Ref())

	// a broken output never breaks the whole search
	searcher.RunCommand = func(name string, args ...string) ([]byte, error) {
		return []byte("not a package table at all\n\n???"), nil
	}
	assert.NotPanics(t, func() {
		result = searcher.Search("jenkins", []string{"npm"})
	})
	assert.Empty(t, result)
}

func TestRepositorySearcherLimitsResults(t *testing.T) {
	lines := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		lines = append(lines, fmt.Sprintf("fake%d - description %d", i, i))
	}
	searcher := &RepositorySearcher{
		Execer: &fakeruntime.FakeExecer{ExpectOS: "linux"},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			return []byte(strings.Join(lines, "\n")), nil
		},
	}
	result := searcher.Search("fake", []string{"apt"})
	assert.Len(t, result, maxResultsPerManager)
}

func TestSearchDefaults(t *testing.T) {
	// a searcher without any configuration falls back to the defaults
	searcher := &RepositorySearcher{}
	searcher.Execer = &fakeruntime.FakeExecer{ExpectOS: "linux"}
	searcher.RunCommand = func(name string, args ...string) ([]byte, error) {
		return []byte(searchOutputs["apt-cache"]), nil
	}
	assert.NotNil(t, searcher.Search("jenkins", []string{"apt"}))

	nilSearcher := &RepositorySearcher{}
	nilSearcher.Execer = nil
	nilSearcher.RunCommand = func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("skip")
	}
	assert.Empty(t, nilSearcher.Search("jenkins", []string{"apt"}))
}
