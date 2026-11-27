// Package remote contains the installer target abstraction. Installs default
// to the local machine, SSH targets download and extract the package locally
// and then copy the binaries to the target server.
package remote

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	// ModeLocal installs on the machine running hd
	ModeLocal = "local"
	// ModeSSH installs on a server reachable over SSH
	ModeSSH = "ssh"
)

// Config describes an installation target
type Config struct {
	Mode     string `json:"mode" yaml:"mode"`
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	User     string `json:"user" yaml:"user"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	KeyFile  string `json:"key_file,omitempty" yaml:"keyFile,omitempty"`
	// Insecure skips the known_hosts verification, it is opt-in and should
	// only be used for ephemeral hosts
	Insecure bool `json:"insecure,omitempty" yaml:"insecure,omitempty"`
	// OS and Arch describe the target, they select the release asset
	OS   string `json:"os" yaml:"os"`
	Arch string `json:"arch" yaml:"arch"`
}

// Normalize fills in the SSH defaults
func (c Config) Normalize() Config {
	if c.Mode == "" {
		c.Mode = ModeLocal
	}
	if c.Port == 0 {
		c.Port = 22
	}
	if c.User == "" {
		c.User = "root"
	}
	if c.OS == "" {
		c.OS = "linux"
	}
	if c.Arch == "" {
		c.Arch = "amd64"
	}
	return c
}

// Validate checks the SSH settings before opening a connection
func (c Config) Validate() error {
	if c.Mode != ModeSSH {
		return nil
	}
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("an SSH host is required when the install mode is ssh")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid SSH port: %d", c.Port)
	}
	if c.Password == "" && c.KeyFile == "" && os.Getenv("SSH_AUTH_SOCK") == "" {
		return fmt.Errorf("an SSH password, a private key or SSH_AUTH_SOCK is required")
	}
	return nil
}

// Executor is the subset of install operations performed on a target server
type Executor interface {
	// Run executes a shell command, stdin may be nil
	Run(command string, stdin io.Reader, stdout, stderr io.Writer) error
	// Copy uploads a local file to an absolute path on the target
	Copy(localPath, remotePath string) error
	Close() error
}

// SSHExecutor runs commands and copies files over an SSH connection
type SSHExecutor struct {
	client *ssh.Client
}

// NewSSHExecutor connects to the target described by cfg
func NewSSHExecutor(ctx context.Context, cfg Config) (*SSHExecutor, error) {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	auth, err := authMethods(cfg)
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := hostKeyCallback(cfg)
	if err != nil {
		return nil, err
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to %s: %v", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
	})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("cannot open an SSH session to %s: %v", addr, err)
	}
	return &SSHExecutor{client: ssh.NewClient(sshConn, chans, reqs)}, nil
}

func authMethods(cfg Config) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if cfg.KeyFile != "" {
		data, err := os.ReadFile(cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read the SSH key: %v", err)
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse the SSH key: %v", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
	}
	if socket := os.Getenv("SSH_AUTH_SOCK"); socket != "" {
		if conn, err := net.Dial("unix", socket); err == nil {
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("no usable SSH authentication method")
	}
	return methods, nil
}

func hostKeyCallback(cfg Config) (ssh.HostKeyCallback, error) {
	if cfg.Insecure {
		return ssh.InsecureIgnoreHostKey(), nil // #nosec G106 -- explicitly requested by the user
	}
	home, err := homedir.Dir()
	if err != nil {
		return nil, err
	}
	knownHosts := filepath.Join(home, ".ssh", "known_hosts")
	if _, err = os.Stat(knownHosts); err != nil {
		return nil, fmt.Errorf("cannot verify the host key: %s does not exist, connect with ssh once or set insecure: true", knownHosts)
	}
	return knownhosts.New(knownHosts)
}

// Run executes the command in a shell on the target server
func (e *SSHExecutor) Run(command string, stdin io.Reader, stdout, stderr io.Writer) error {
	session, err := e.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	session.Stdin = stdin
	session.Stdout = stdout
	session.Stderr = stderr
	return session.Run(command)
}

// Copy uploads the local file through the SSH session, it works without an
// SFTP server on the target
func (e *SSHExecutor) Copy(localPath, remotePath string) error {
	if !filepath.IsAbs(remotePath) && !strings.HasPrefix(remotePath, "/") {
		return fmt.Errorf("the remote path must be absolute: %s", remotePath)
	}
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return e.Run("cat > "+Quote(remotePath), file, io.Discard, io.Discard)
}

// Close closes the SSH connection
func (e *SSHExecutor) Close() error {
	return e.client.Close()
}

// Quote escapes a value for use in a remote shell command
func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
