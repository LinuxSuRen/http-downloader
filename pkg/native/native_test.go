package native

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/stretchr/testify/assert"
)

// recordingExecer records the executed commands on top of a fake execer
type recordingExecer struct {
	fakeruntime.FakeExecer

	mu    sync.Mutex
	calls []string
}

func (r *recordingExecer) record(format string, args ...string) {
	line := strings.TrimSpace(fmt.Sprintf(format, args[0], strings.Join(args[1:], " ")))
	r.mu.Lock()
	r.calls = append(r.calls, line)
	r.mu.Unlock()
}

func (r *recordingExecer) RunCommand(name string, args ...string) error {
	r.record("%s %s", append([]string{name}, args...)...)
	return r.ExpectError
}

func (r *recordingExecer) RunCommandWithSudo(name string, args ...string) error {
	r.record("sudo %s %s", append([]string{name}, args...)...)
	return r.ExpectError
}

// lookPathWrapper overrides the look path behavior of an execer
type lookPathWrapper struct {
	fakeruntime.Execer
	lookPath func(name string) (string, error)
}

func (l *lookPathWrapper) LookPath(name string) (string, error) {
	return l.lookPath(name)
}

func allMissing(name string) (string, error) {
	return "", errors.New("not found")
}

// absolute replaces the file placeholder in the expected command lines with
// its absolute path, the installer always passes an absolute path to the
// system package tools
func absolute(t *testing.T, file string, lines []string) (result []string) {
	abs, err := filepath.Abs(file)
	assert.NoError(t, err)
	for i := range lines {
		result = append(result, strings.ReplaceAll(lines[i], file, abs))
	}
	return
}

func TestDetect(t *testing.T) {
	for i, tt := range []struct {
		name   string
		expect Format
	}{{
		name:   "hd_1.0.0_amd64.deb",
		expect: FormatDeb,
	}, {
		name:   "hd_1.0.0_x86_64.rpm",
		expect: FormatRPM,
	}, {
		name:   "hd_1.0.0_windows_amd64.msi",
		expect: FormatMSI,
	}, {
		name:   "hd_1.0.0_darwin_universal.pkg",
		expect: FormatPkg,
	}, {
		name:   "hd_1.0.0_x86_64.apk",
		expect: FormatApk,
	}, {
		name:   "hd_1.0.0_x86_64.pkg.tar.zst",
		expect: FormatPacman,
	}, {
		name:   "hd_1.0.0_x86_64.pkg.tar.xz",
		expect: FormatPacman,
	}, {
		name:   "hd_1.0.0_x86_64.pkg.tar.gz",
		expect: FormatPacman,
	}, {
		name:   "HD_1.0.0_AMD64.DEB",
		expect: FormatDeb,
	}, {
		name:   "hd_1.0.0_linux_amd64.tar.gz",
		expect: "",
	}, {
		name:   "hd.zip",
		expect: "",
	}, {
		name:   "",
		expect: "",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, Detect(tt.name),
				"failed with case [%d] - [%s]", i, tt.name)
			assert.Equal(t, tt.expect != "", IsInstallerFile(tt.name))
		})
	}
}

func TestInstallNotSupported(t *testing.T) {
	// not a native installer package
	installer := &Installer{
		Execer: &fakeruntime.FakeExecer{ExpectOS: "linux"},
	}
	assert.Error(t, installer.Install("hd.zip"))

	// no system package tool available at all
	installer = &Installer{
		Execer: &lookPathWrapper{
			Execer:   &fakeruntime.FakeExecer{ExpectOS: "linux"},
			lookPath: allMissing,
		},
		Root: true,
	}
	assert.Error(t, installer.Install("hd.deb"))
	assert.Error(t, installer.Install("hd.rpm"))
}

func TestInstallCommands(t *testing.T) {
	for i, tt := range []struct {
		name     string
		os       string
		file     string
		root     bool
		lookPath func(name string) (string, error)
		expect   []string
	}{{
		name: "deb with apt-get as root",
		os:   "linux",
		file: "hd.deb",
		root: true,
		expect: []string{
			"apt-get install -y hd.deb",
		},
	}, {
		name: "deb with dpkg only",
		os:   "linux",
		file: "hd.deb",
		root: true,
		lookPath: func(name string) (string, error) {
			if name == "dpkg" {
				return name, nil
			}
			return "", errors.New("not found")
		},
		expect: []string{
			"dpkg -i hd.deb",
		},
	}, {
		name: "rpm with dnf",
		os:   "linux",
		file: "hd.rpm",
		root: true,
		expect: []string{
			"dnf install -y hd.rpm",
		},
	}, {
		name: "rpm with yum",
		os:   "linux",
		file: "hd.rpm",
		root: true,
		lookPath: func(name string) (string, error) {
			if name == "yum" || name == "rpm" {
				return name, nil
			}
			return "", errors.New("not found")
		},
		expect: []string{
			"yum localinstall -y hd.rpm",
		},
	}, {
		name: "rpm with the low level rpm only",
		os:   "linux",
		file: "hd.rpm",
		root: true,
		lookPath: func(name string) (string, error) {
			if name == "rpm" {
				return name, nil
			}
			return "", errors.New("not found")
		},
		expect: []string{
			"rpm -i --replacepkgs hd.rpm",
		},
	}, {
		name: "apk",
		os:   "linux",
		file: "hd.apk",
		root: true,
		expect: []string{
			"apk add --allow-untrusted hd.apk",
		},
	}, {
		name: "pacman file",
		os:   "linux",
		file: "hd.pkg.tar.zst",
		root: true,
		expect: []string{
			"pacman -U --noconfirm hd.pkg.tar.zst",
		},
	}, {
		name: "macOS pkg",
		os:   "darwin",
		file: "hd.pkg",
		root: true,
		expect: []string{
			"installer -pkg hd.pkg -target /",
		},
	}, {
		name: "deb as a non-root user",
		os:   "linux",
		file: "hd.deb",
		expect: []string{
			"sudo apt-get install -y hd.deb",
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			execer := &recordingExecer{
				FakeExecer: fakeruntime.FakeExecer{ExpectOS: tt.os},
			}

			installer := &Installer{
				Execer: execer,
				Root:   tt.root,
				euid:   func() int { return 1 },
			}
			if tt.lookPath != nil {
				// override the look path behavior via the wrapper
				installer.Execer = &lookPathWrapper{
					Execer:   execer,
					lookPath: tt.lookPath,
				}
			}

			assert.NoError(t, installer.Install(tt.file))
			assert.Equal(t, absolute(t, tt.file, tt.expect), execer.calls,
				"failed with case [%d] - [%s]", i, tt.name)
		})
	}
}

func TestInstallMSI(t *testing.T) {
	execer := &recordingExecer{
		FakeExecer: fakeruntime.FakeExecer{ExpectOS: "windows"},
	}
	installer := &Installer{Execer: execer}

	assert.NoError(t, installer.Install("hd.msi"))
	assert.Equal(t, absolute(t, "hd.msi", []string{"msiexec /i hd.msi /passive /norestart"}), execer.calls)

	// msi is not supported on the other platforms
	execer.FakeExecer.ExpectOS = "linux"
	assert.Error(t, installer.Install("hd.msi"))
}

func TestInstallPkgOSRestriction(t *testing.T) {
	execer := &recordingExecer{
		FakeExecer: fakeruntime.FakeExecer{ExpectOS: "linux"},
	}
	installer := &Installer{Execer: execer, Root: true}

	// a macOS pkg file cannot be installed on Linux
	assert.Error(t, installer.Install("hd.pkg"))
	assert.Empty(t, execer.calls)
}

func TestNeedSudo(t *testing.T) {
	execer := &fakeruntime.FakeExecer{ExpectOS: "linux"}

	// the root detection is injectable
	installer := &Installer{Execer: execer, euid: func() int { return 0 }}
	assert.False(t, installer.needSudo())

	installer = &Installer{Execer: execer, euid: func() int { return 1000 }}
	assert.True(t, installer.needSudo())

	installer = &Installer{Execer: execer, Root: true, euid: func() int { return 1000 }}
	assert.False(t, installer.needSudo())

	// sudo is never needed on Windows
	execer.ExpectOS = "windows"
	installer = &Installer{Execer: execer, euid: func() int { return 1000 }}
	assert.False(t, installer.needSudo())
}
