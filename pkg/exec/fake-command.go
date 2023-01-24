package exec

import "io"

// FakeExecer is for the unit test purposes
type FakeExecer struct {
	ExpectError  error
	ExpectOutput string
	ExpectOS     string
	ExpectArch   string
}

// LookPath is a fake method
func (f FakeExecer) LookPath(path string) (string, error) {
	return "", f.ExpectError
}

// Command is a fake method
func (f FakeExecer) Command(name string, arg ...string) ([]byte, error) {
	return []byte(f.ExpectOutput), f.ExpectError
}

// RunCommand runs a command
func (f FakeExecer) RunCommand(name string, arg ...string) error {
	return f.ExpectError
}

// RunCommandWithIO is a fake method
func (f FakeExecer) RunCommandWithIO(name, dir string, stdout, stderr io.Writer, args ...string) error {
	return f.ExpectError
}

// OS returns the os name
func (f FakeExecer) OS() string {
	return f.ExpectOS
}

// Arch returns the os arch
func (f FakeExecer) Arch() string {
	return f.ExpectArch
}
