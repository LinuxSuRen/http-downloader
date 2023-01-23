package exec

// FakeExecer is for the unit test purposes
type FakeExecer struct {
	ExpectError  error
	ExpectOutput string
}

// LookPath is a fake method
func (f FakeExecer) LookPath(path string) (string, error) {
	return "", f.ExpectError
}

// Command ia fake method
func (f FakeExecer) Command(name string, arg ...string) ([]byte, error) {
	return []byte(f.ExpectOutput), f.ExpectError
}
