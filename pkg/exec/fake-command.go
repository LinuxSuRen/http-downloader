package exec

// FakeExecer is for the unit test purposes
type FakeExecer struct {
	ExpectError error
}

// LookPath is a fake method
func (f FakeExecer) LookPath(path string) (string, error) {
	return "", f.ExpectError
}
