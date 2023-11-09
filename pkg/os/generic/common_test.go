package generic

import (
	"testing"

	fakeruntime "github.com/linuxsuren/go-fake-runtime"
	"github.com/stretchr/testify/assert"
)

func TestSliceReplace(t *testing.T) {
	installer := &CommonInstaller{}
	installer.SetURLReplace(map[string]string{
		"https://raw.githubusercontent.com": "https://gh-proxy.com/https://raw.githubusercontent.com",
	})

	// a normal case
	result := installer.sliceReplace([]string{
		"abc",
		"https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh",
	})
	assert.Equal(t, []string{"abc",
		"https://gh-proxy.com/https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"}, result)

	// an empty slice
	noProxyInstaller := &CommonInstaller{}
	assert.Equal(t, []string{"abc"}, noProxyInstaller.sliceReplace([]string{"abc"}))
}

func TestCommonInstaller(t *testing.T) {
	installer := &CommonInstaller{
		Execer: fakeruntime.FakeExecer{},
	}
	assert.Nil(t, installer.Install())
	assert.Nil(t, installer.Uninstall())
	assert.True(t, installer.Available())
	assert.Nil(t, installer.Stop())
	assert.Nil(t, installer.Start())

	ok, err := installer.WaitForStart()
	assert.True(t, ok)
	assert.Nil(t, err)
}
