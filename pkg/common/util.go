package common

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// GetOrDefault returns the value or a default value from a map
func GetOrDefault(key, def string, data map[string]string) (result string) {
	var ok bool
	if result, ok = data[key]; !ok {
		result = def
	}
	return
}

// GetReplacement returns a string which replace via a map
func GetReplacement(key string, data map[string]string) (result string) {
	return GetOrDefault(key, key, data)
}

// PathExists checks if the target path exist or not
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

const (
	// PrivateFileMode grants owner to read/write a file.
	PrivateFileMode = 0600
	// PrivateDirMode grants owner to make/remove files inside the directory.
	PrivateDirMode = 0700
)

// IsDirWriteable checks if dir is writable by writing and removing a file
// to dir. It returns nil if dir is writable.
func IsDirWriteable(dir string) error {
	f := filepath.Join(dir, ".touch")
	if err := os.WriteFile(f, []byte(""), PrivateFileMode); err != nil {
		return err
	}
	return os.Remove(f)
}

// CheckDirPermission checks permission on an existing dir.
// Returns error if dir is empty or exist with a different permission than specified.
func CheckDirPermission(dir string, perm os.FileMode) error {
	if !Exist(dir) {
		return fmt.Errorf("directory %q empty, cannot check permission", dir)
	}
	// check the existing permission on the directory
	dirInfo, err := os.Stat(dir)
	if err != nil {
		return err
	}
	dirMode := dirInfo.Mode().Perm()
	if dirMode != perm {
		err = fmt.Errorf("directory %q exist, but the permission is %q. The recommended permission is %q to prevent possible unprivileged access to the data", dir, dirInfo.Mode(), os.FileMode(PrivateDirMode))
		return err
	}
	return nil
}

// Exist returns true if a file or directory exists.
func Exist(name string) bool {
	ok, _ := PathExists(name)
	return ok
}

// ParseVersionNum split version from release or tag
func ParseVersionNum(release string) string {
	return regexp.MustCompile(`^.*v`).ReplaceAllString(release, "")
}

// GetEnvironment retrieves the value of the environment variable named by the key.
// If the environment doesn't exist, we will lookup for alternative environment variables
// until we find an environment. Return empty environment value while no environment variables found.
func GetEnvironment(key string, alternativeKeys ...string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	for _, alternativeKey := range alternativeKeys {
		if value, exists := os.LookupEnv(alternativeKey); exists {
			return value
		}
	}
	return ""
}
