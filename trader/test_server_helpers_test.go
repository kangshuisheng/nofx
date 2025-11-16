package trader

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestHTTPServer tries to start a loopback HTTP server. In sandboxed
// environments where binding sockets is not allowed, it skips the invoking test.
func newTestHTTPServer(tb testing.TB, handler http.Handler) *httptest.Server {
	tb.Helper()

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		ln6, err6 := net.Listen("tcp6", "[::1]:0")
		if err6 != nil {
			tb.Skipf("跳過測試：無法在本地啟動HTTP服務 (IPv4=%v, IPv6=%v)", err, err6)
		}
		ln = ln6
	}

	server := &httptest.Server{
		Listener: ln,
		Config:   &http.Server{Handler: handler},
	}
	server.Start()
	return server
}
