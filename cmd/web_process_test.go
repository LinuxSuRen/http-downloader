package cmd

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	// processHelperEnv and processHelperMode drive TestHelperProcess, the
	// helper acts as the tool process started by the web console tests
	processHelperEnv  = "HD_TEST_PROCESS_HELPER"
	processHelperMode = "HD_TEST_PROCESS_MODE"

	processHelperModeHang = "hang"
	processHelperModeFail = "fail"
)

func TestSplitCommand(t *testing.T) {
	args, err := splitCommand("server --port 8080", "fakeweb")
	assert.NoError(t, err)
	assert.Equal(t, []string{"server", "--port", "8080"}, args)

	// a command copied from a doc keeps its tool name prefix
	args, err = splitCommand("fakeweb server --port 8080", "fakeweb")
	assert.NoError(t, err)
	assert.Equal(t, []string{"server", "--port", "8080"}, args)

	args, err = splitCommand("/usr/local/bin/fakeweb serve", "fakeweb")
	assert.NoError(t, err)
	assert.Equal(t, []string{"serve"}, args)

	// quoted segments keep their spaces
	args, err = splitCommand("serve --name \"fake name\" --debug", "fakeweb")
	assert.NoError(t, err)
	assert.Equal(t, []string{"serve", "--name", "fake name", "--debug"}, args)

	args, err = splitCommand("", "fakeweb")
	assert.NoError(t, err)
	assert.Empty(t, args)

	// the tool name of another tool is a real argument
	args, err = splitCommand("other-tool serve", "fakeweb")
	assert.NoError(t, err)
	assert.Equal(t, []string{"other-tool", "serve"}, args)

	args, err = splitCommand("serve --name 'unclosed", "fakeweb")
	assert.ErrorIs(t, err, errUnbalancedQuotes)
	assert.Empty(t, args)
}

func TestParseLsofPorts(t *testing.T) {
	out := "p123\nf456\nn127.0.0.1:8080\nn*:9000\nn[::1]:7000\nnfe80::1%lo0:notaport\nx500\n"
	assert.Equal(t, []int{8080, 9000, 7000}, parseLsofPorts(out))
	assert.Empty(t, parseLsofPorts("p123\n"))
}

func TestParseNetstatPorts(t *testing.T) {
	out := `
  Proto  Local Address          Foreign Address        State           PID
  TCP    127.0.0.1:8080         0.0.0.0:0              LISTENING       1234
  TCP    127.0.0.1:9000         0.0.0.0:0              LISTENING       4321
  TCP    127.0.0.1:7000         0.0.0.0:0              ESTABLISHED     1234
  TCP    [::1]:7500             [::]:0                 LISTENING       1234
`
	assert.Equal(t, []int{8080, 7500}, parseNetstatPorts(out, 1234))
}

func TestParseProcNetTable(t *testing.T) {
	out := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 11111 1 0000000000000000 100 0 0 10 0\n" +
		"   1: 0100007F:2382 00000000:0000 01 00000000:00000000 00:00000000 00000000  1000        0 22222 1 0000000000000000 100 0 0 10 0\n" +
		"   2: 00000000:1F91 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 33333 1 0000000000000000 100 0 0 10 0\n"
	assert.Equal(t, []int{8080, 8081}, parseProcNetTable(out, map[string]bool{"11111": true, "33333": true}))
}

func TestListeningPortsAndDetectWeb(t *testing.T) {
	// the test binary itself listens on the httptest port, the detection
	// finds it as the endpoint of the process
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(ts.Close)

	port := 0
	_, portStr, err := net.SplitHostPort(strings.TrimPrefix(ts.URL, "http://"))
	assert.NoError(t, err)
	port, err = strconv.Atoi(portStr)
	assert.NoError(t, err)
	assert.Contains(t, listeningPorts(os.Getpid()), port)

	server := newWebServer(nil)
	url, ok := server.detectWebURL(os.Getpid())
	assert.True(t, ok)
	assert.Equal(t, "http://localhost:"+portStr, url)
}
