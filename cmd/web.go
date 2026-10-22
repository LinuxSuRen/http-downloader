package cmd

import (
	"context"
	// _ "embed" is required by the go:embed directive below to serve the web console
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

//go:embed web/index.html
var webIndexHTML []byte

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
	return
}

type webOption struct {
	Listen string
	Port   int
	server *webServer
}

func (o *webOption) runE(cmd *cobra.Command, args []string) (err error) {
	addr := fmt.Sprintf("%s:%d", o.Listen, o.Port)
	cmd.Printf("web server is running at http://%s\n", addr)
	// use an explicit http.Server so that timeouts are always set
	server := &http.Server{
		Addr:              addr,
		Handler:           o.server.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	err = server.ListenAndServe()
	return
}

// toolItem is the API view of an installed tool
type toolItem struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Org         string `json:"org,omitempty"`
	Repo        string `json:"repo,omitempty"`
	InstalledAt string `json:"installed_at,omitempty"`
	Directory   string `json:"directory,omitempty"`
	Upgradable  bool   `json:"upgradable"`
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
