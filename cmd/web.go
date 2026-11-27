package cmd

import (
	"context"
	// _ "embed" is required by the go:embed directive below to serve the web console
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/linuxsuren/http-downloader/docs"
	"github.com/spf13/cobra"
)

//go:embed web/index.html
var webIndexHTML []byte

// handleLogo serves the project logo, the single source of it lives in
// docs/logo.svg
func (s *webServer) handleLogo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only GET is supported")
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "max-age=86400")
	_, _ = w.Write(docs.LogoSVG)
}

func newWebCmd(ctx context.Context) (cmd *cobra.Command) {
	opt := &webOption{
		server: newWebServer(ctx),
	}
	cmd = &cobra.Command{
		Use:     "web",
		Short:   "Start a web server to manage the installed tools",
		Example: "hd web --port 8080",
		RunE:    opt.runE,
		GroupID: coreGroup.ID,
	}
	flags := cmd.Flags()
	flags.StringVarP(&opt.Listen, "listen", "", "127.0.0.1", "The address to listen on")
	flags.IntVarP(&opt.Port, "port", "p", 8080, "The port to listen on")
	flags.BoolVar(&opt.PortDrift, "port-drift", true, "Try the next ports when the requested port is already in use")
	flags.IntVar(&opt.PortDriftAttempts, "port-drift-attempts", 100, "How many consecutive ports to try when port drift is enabled")
	return
}

const maxTCPPort = 65535

type webOption struct {
	Listen            string
	Port              int
	PortDrift         bool
	PortDriftAttempts int
	server            *webServer

	// listen and serve are replaceable so the port selection can be
	// tested without occupying real ports or starting the console
	listen func(network, address string) (net.Listener, error)
	serve  func(*http.Server, net.Listener) error
}

// listenWithDrift binds the requested port, moving on to the following
// ports when one is already occupied. Binding directly (instead of
// checking and then listening) avoids a race with other processes.
func (o *webOption) listenWithDrift() (listener net.Listener, actualPort int, err error) {
	if o.Port < 1 || o.Port > maxTCPPort {
		return nil, 0, fmt.Errorf("invalid port: %d", o.Port)
	}

	attempts := o.PortDriftAttempts
	if attempts < 1 {
		attempts = 1
	}
	if o.Port+attempts-1 > maxTCPPort {
		attempts = maxTCPPort - o.Port + 1
	}

	listen := o.listen
	if listen == nil {
		listen = net.Listen
	}

	lastPort := o.Port
	for i := 0; i < attempts; i++ {
		lastPort = o.Port + i
		listener, err = listen("tcp", net.JoinHostPort(o.Listen, strconv.Itoa(lastPort)))
		if err == nil {
			return listener, lastPort, nil
		}
		if !o.PortDrift || !isAddrInUse(err) {
			return nil, 0, err
		}
	}

	return nil, 0, fmt.Errorf("no available port in range %d-%d: %w", o.Port, lastPort, err)
}

func (o *webOption) runE(cmd *cobra.Command, args []string) (err error) {
	listener, actualPort, err := o.listenWithDrift()
	if err != nil {
		return err
	}
	if actualPort != o.Port {
		cmd.Printf("port %d is in use, drifting to %d\n", o.Port, actualPort)
	}
	cmd.Printf("web server is running at http://%s\n", listener.Addr().String())
	// the web service detection must not report the console itself as the
	// endpoint of a started tool; it must use the port we ended up with
	o.server.selfPort = actualPort
	// use an explicit http.Server so that timeouts are always set
	server := &http.Server{
		Handler:           o.server.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	serve := o.serve
	if serve == nil {
		serve = (*http.Server).Serve
	}
	err = serve(server, listener)
	return
}

// toolItem is the API view of an installed tool
type toolItem struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Org         string `json:"org,omitempty"`
	Repo        string `json:"repo,omitempty"`
	Source      string `json:"source,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Host        string `json:"host,omitempty"`
	InstalledAt string `json:"installed_at,omitempty"`
	Directory   string `json:"directory,omitempty"`
	Upgradable  bool   `json:"upgradable"`
	// LaunchCommand is the argument string the web console passes to the
	// binary when the tool starts
	LaunchCommand string `json:"launch_command,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError returns a stable error code with a human readable message,
// the technical root cause only goes into the server log
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

func decodeJSON(r *http.Request, target interface{}) (err error) {
	return json.NewDecoder(r.Body).Decode(target)
}
