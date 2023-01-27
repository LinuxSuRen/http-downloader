package core

// FakeRegistry is a fake registry which only for the test purpose
type FakeRegistry struct {
	data map[string]Installer
}

// Registry puts the registry to memory
func (r *FakeRegistry) Registry(id string, installer Installer) {
	if r.data == nil {
		r.data = map[string]Installer{}
	}
	r.data[id] = installer
}

// Walk allows to iterate all the installers
func (r *FakeRegistry) Walk(walkFunc func(string, Installer)) {
	if r.data != nil {
		for id, installer := range r.data {
			walkFunc(id, installer)
		}
	}
}
