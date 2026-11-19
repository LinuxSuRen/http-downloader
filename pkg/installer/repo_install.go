package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	sysos "os"
	"path/filepath"
	"runtime"
	"strings"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
)

// ErrUnknownSource marks an address whose first segment is no registered
// repo source, the address may belong to hd-home instead
var ErrUnknownSource = errors.New("unknown repo source")

// FindRepoPackage resolves the exact package behind a source/name
// address, e.g. debian-stable/curl. The sources register in hd-home,
// which makes the address deterministic: a registered source name wins
// over a same named hd-home organization.
func FindRepoPackage(ctx context.Context, configDir, sourceName, pkgName string) (source *RepoSource, pkg *RepoPackage, err error) {
	sources := AllRepoSources(configDir)
	for i := range sources {
		if sources[i].Name != sourceName {
			continue
		}
		// an explicit address resolves on any platform, the install
		// reports the mismatch instead of falling back to hd-home
		if !sources[i].servesPlatform(runtime.GOOS) {
			return nil, nil, fmt.Errorf("source %s does not serve %s", sourceName, runtime.GOOS)
		}
		hits, searchErr := sources[i].Search(ctx, configDir, pkgName)
		if searchErr != nil {
			return nil, nil, searchErr
		}
		for j := range hits {
			if hits[j].Name == pkgName {
				return &sources[i], &hits[j], nil
			}
		}
		return nil, nil, fmt.Errorf("package %s not found in source %s", pkgName, sourceName)
	}
	return nil, nil, ErrUnknownSource
}

// InstallRepoPackage installs a package of a repo source through the
// native channel of the source
func InstallRepoPackage(ctx context.Context, source RepoSource, pkg RepoPackage,
	execer fakeruntime.Execer) (err error) {
	switch source.Kind {
	case "apt-index":
		return installAptPackage(ctx, source, pkg, execer)
	default:
		return fmt.Errorf("installing from a %s source is not supported yet", source.Kind)
	}
}

// installAptPackage downloads the package file of the apt index, checks
// its checksum and hands it to dpkg, the official tooling owns the
// dependency handling
func installAptPackage(ctx context.Context, source RepoSource, pkg RepoPackage,
	execer fakeruntime.Execer) (err error) {
	if _, err = execer.LookPath("dpkg"); err != nil {
		return fmt.Errorf("dpkg is not available, please install it first")
	}

	file, err := sysos.CreateTemp("", "hd-repo-*.deb")
	if err != nil {
		return
	}
	target := filepath.ToSlash(file.Name())
	defer func() { _ = sysos.Remove(target) }()

	if err = downloadAndVerify(ctx, pkg, file); err != nil {
		_ = file.Close()
		return fmt.Errorf("cannot download %s from %s, error: %v", pkg.Name, source.Name, err)
	}
	if err = file.Close(); err != nil {
		return
	}
	return runWithRoot(execer, "dpkg", "-i", target)
}

// downloadAndVerify streams the package file into the writer and
// compares the SHA256 checksum the index declared
func downloadAndVerify(ctx context.Context, pkg RepoPackage, target io.Writer) (err error) {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, pkg.URL, nil)
	if reqErr != nil {
		return reqErr
	}
	resp, err := repoHTTPClient.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	hash := sha256.New()
	if _, err = io.Copy(io.MultiWriter(target, hash), resp.Body); err != nil {
		return
	}
	if pkg.SHA256 != "" && !strings.EqualFold(pkg.SHA256, hex.EncodeToString(hash.Sum(nil))) {
		return fmt.Errorf("checksum mismatch, expected %s", pkg.SHA256)
	}
	return
}

// runWithRoot runs the command directly under root, otherwise through
// the sudo channel of the executer
func runWithRoot(execer fakeruntime.Execer, name string, args ...string) error {
	if sysos.Geteuid() == 0 {
		return execer.RunCommand(name, args...)
	}
	return execer.RunCommandWithSudo(name, args...)
}

// UninstallRepoPackage removes a package which was installed from a repo
// source, it tells by the return whether the source owns a channel
func UninstallRepoPackage(configDir string, execer fakeruntime.Execer, tool InstalledPackage) (handled bool, err error) {
	if tool.Source == "" {
		return false, nil
	}
	for _, source := range AllRepoSources(configDir) {
		if source.Name != tool.Source {
			continue
		}
		switch source.Kind {
		case "apt-index":
			if _, lookErr := execer.LookPath("dpkg"); lookErr != nil {
				return true, fmt.Errorf("dpkg is not available, please remove %s manually", tool.Name)
			}
			return true, runWithRoot(execer, "dpkg", "-r", tool.Name)
		default:
			return false, nil
		}
	}
	return false, nil
}
