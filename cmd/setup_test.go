package cmd

import (
	"context"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	expect "github.com/Netflix/go-expect"
	pseudotty "github.com/creack/pty"
	"github.com/hinshun/vt10x"
	"github.com/linuxsuren/http-downloader/pkg/log"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func Test_newSetupCommand(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		RunTest(t, func(c expectConsole) {
			c.ExpectString("Select proxy-github")
			// c.Send("99988866")
			c.SendLine("")

			c.ExpectString("Select provider")
			c.Send("gitee")
			c.SendLine("")
			c.ExpectEOF()
		}, func(tr terminal.Stdio) error {
			fs := afero.NewMemMapFs()
			v := viper.New()
			v.SetFs(fs)
			v.Set("provider", "github")

			cmd := newSetupCommand(v, tr)
			assert.Equal(t, "setup", cmd.Name())

			err := cmd.Execute()
			assert.Nil(t, err)
			// assert.Equal(t, "gh.api.99988866.xyz", v.GetString("proxy-github"))
			assert.Equal(t, "gitee", v.GetString("provider"))
			return err
		})
	})

	t.Run("test the default value", func(t *testing.T) {
		RunTest(t, func(c expectConsole) {
			c.ExpectString("Select proxy-github")
			c.SendLine("")

			c.ExpectString("Select provider")
			c.SendLine("")
			c.ExpectEOF()
		}, func(tr terminal.Stdio) error {
			fs := afero.NewMemMapFs()
			v := viper.New()
			v.SetFs(fs)
			v.Set("provider", "gitee")
			v.Set("proxy-github", "gh.api.99988866.xyz")

			cmd := newSetupCommand(v, tr)
			assert.Equal(t, "setup", cmd.Name())

			err := cmd.Execute()
			assert.Nil(t, err)
			assert.Equal(t, "gh.api.99988866.xyz", v.GetString("proxy-github"))
			assert.Equal(t, "gitee", v.GetString("provider"))
			return err
		})
	})

	t.Run("setup with given flags", func(t *testing.T) {
		RunTest(t, func(c expectConsole) {
		}, func(tr terminal.Stdio) error {
			fs := afero.NewMemMapFs()
			v := viper.New()
			v.SetFs(fs)
			v.Set("provider", "gitee")
			v.Set("proxy-github", "gh.api.99988866.xyz")

			cmd := newSetupCommand(v, tr)
			assert.Equal(t, "setup", cmd.Name())
			cmd.SetArgs([]string{"--proxy", "fake.com", "--provider", "fake"})
			cmd.SetContext(log.NewContextWithLogger(context.Background(), 0))

			err := cmd.Execute()
			assert.Nil(t, err)
			assert.Equal(t, "fake.com", v.GetString("proxy-github"))
			assert.Equal(t, "fake", v.GetString("provider"))
			return err
		})
	})
}

func TestSelectFromList(t *testing.T) {
	RunTest(t, func(c expectConsole) {
		c.ExpectString("title")
		c.SendLine(string(terminal.KeyArrowDown))
		c.SendLine("")
		c.ExpectEOF()
	}, func(tr terminal.Stdio) error {
		val, err := selectFromList([]string{"one", "two", "three"}, "", "title", tr)
		assert.Equal(t, "two", val)
		return err
	})
}

type expectConsole interface {
	ExpectString(string)
	ExpectEOF()
	SendLine(string)
	Send(string)
}

func RunTest(t *testing.T, procedure func(expectConsole), test func(terminal.Stdio) error) {
	t.Helper()
	t.Parallel()

	pty, tty, err := pseudotty.Open()
	if err != nil {
		t.Fatalf("failed to open pseudotty: %v", err)
	}

	term := vt10x.New(vt10x.WithWriter(tty))
	c, err := expect.NewConsole(expect.WithStdin(pty), expect.WithStdout(term), expect.WithCloser(pty, tty))
	if err != nil {
		t.Fatalf("failed to create console: %v", err)
	}
	defer c.Close()

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		procedure(&consoleWithErrorHandling{console: c, t: t})
	}()

	stdio := terminal.Stdio{In: c.Tty(), Out: c.Tty(), Err: c.Tty()}
	if err := test(stdio); err != nil {
		t.Error(err)
	}

	if err := c.Tty().Close(); err != nil {
		t.Errorf("error closing Tty: %v", err)
	}
	<-donec
}

type consoleWithErrorHandling struct {
	console *expect.Console
	t       *testing.T
}

func (c *consoleWithErrorHandling) ExpectString(s string) {
	if _, err := c.console.ExpectString(s); err != nil {
		c.t.Helper()
		c.t.Fatalf("ExpectString(%q) = %v", s, err)
	}
}

func (c *consoleWithErrorHandling) SendLine(s string) {
	if _, err := c.console.SendLine(s); err != nil {
		c.t.Helper()
		c.t.Fatalf("SendLine(%q) = %v", s, err)
	}
}

func (c *consoleWithErrorHandling) Send(s string) {
	if _, err := c.console.Send(s); err != nil {
		c.t.Helper()
		c.t.Fatalf("Send(%q) = %v", s, err)
	}
}

func (c *consoleWithErrorHandling) ExpectEOF() {
	if _, err := c.console.ExpectEOF(); err != nil {
		c.t.Helper()
		c.t.Fatalf("ExpectEOF() = %v", err)
	}
}
