package version

import (
	"regexp"
	"strings"

	"github.com/blang/semver/v4"
)

// GetVersion returns the version string from a string
func GetVersion(output string) string {
	ver, _ := GetSemVersion(output)
	return ver.String()
}

// GreatThan return true if target is great than output
func GreatThan(target, output string) (ok bool) {
	var (
		targetVer  semver.Version
		currentVer semver.Version
		err        error
	)

	if targetVer, err = semver.ParseTolerant(target); err == nil {
		if currentVer, err = GetSemVersion(output); err == nil {
			ok = targetVer.GT(currentVer)
		}
	}
	return
}

// GetSemVersion parses the output and returns the semversion
func GetSemVersion(output string) (semVersion semver.Version, err error) {
	var verReg *regexp.Regexp
	verReg, err = regexp.Compile(`(v\d.+\d+.\d+)|(\d.+\d+.\d+)`)
	if err == nil {
		for _, line := range strings.Split(output, "\n") {
			line = verReg.FindString(line)
			if line == "" {
				continue
			}

			semVersion, err = semver.ParseTolerant(line)
			if err == nil {
				break
			}
		}
	}
	return
}
