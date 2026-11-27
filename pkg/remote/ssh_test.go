package remote

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigNormalize(t *testing.T) {
	cfg := Config{}.Normalize()
	assert.Equal(t, ModeLocal, cfg.Mode)
	assert.Equal(t, 22, cfg.Port)
	assert.Equal(t, "root", cfg.User)
	assert.Equal(t, "linux", cfg.OS)
	assert.Equal(t, "amd64", cfg.Arch)

	custom := Config{Mode: ModeSSH, Port: 2222, User: "hd", OS: "darwin", Arch: "arm64"}.Normalize()
	assert.Equal(t, 2222, custom.Port)
	assert.Equal(t, "hd", custom.User)
	assert.Equal(t, "darwin", custom.OS)
	assert.Equal(t, "arm64", custom.Arch)
}

func TestConfigValidate(t *testing.T) {
	t.Run("local mode needs nothing", func(t *testing.T) {
		assert.NoError(t, Config{}.Validate())
	})

	t.Run("ssh needs a host", func(t *testing.T) {
		err := Config{Mode: ModeSSH}.Normalize().Validate()
		assert.ErrorContains(t, err, "host is required")
	})

	t.Run("ssh needs credentials", func(t *testing.T) {
		t.Setenv("SSH_AUTH_SOCK", "")
		err := Config{Mode: ModeSSH, Host: "example.com"}.Normalize().Validate()
		assert.ErrorContains(t, err, "SSH_AUTH_SOCK")
	})

	t.Run("ssh with a password", func(t *testing.T) {
		cfg := Config{Mode: ModeSSH, Host: "example.com", Password: "secret"}.Normalize()
		assert.NoError(t, cfg.Validate())
	})

	t.Run("invalid port", func(t *testing.T) {
		cfg := Config{Mode: ModeSSH, Host: "example.com", Password: "secret", Port: 70000}
		assert.ErrorContains(t, cfg.Validate(), "invalid SSH port")
	})
}

func TestQuote(t *testing.T) {
	assert.Equal(t, "'/usr/local/bin'", Quote("/usr/local/bin"))
	assert.Equal(t, `'it'\''s'`, Quote("it's"))
	assert.Equal(t, "'C:\\tools\\bin'", Quote(`C:\tools\bin`))
}

func TestNewSSHExecutorRejectsInvalidConfig(t *testing.T) {
	_, err := NewSSHExecutor(nil, Config{Mode: ModeSSH})
	assert.Error(t, err)
}
