package claudecode

import (
	"fmt"
	"net"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// newChromeTransport returns an http.RoundTripper that mimics Chrome's TLS fingerprint.
// Cloudflare blocks Go's default crypto/tls fingerprint on claude.ai POST endpoints.
// Using utls with Chrome's ClientHello spec passes Cloudflare's bot detection.
//
// For HTTPS requests: dials with utls, negotiates ALPN (h2), and uses http2.Transport.
// For HTTP requests (tests): delegates to the standard http.Transport.
func newChromeTransport() http.RoundTripper {
	return &chromeTransport{
		h1: &http.Transport{
			MaxIdleConns:    100,
			IdleConnTimeout: 90 * time.Second,
		},
	}
}

type chromeTransport struct {
	h1 *http.Transport
}

func (t *chromeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// For non-TLS (tests), use standard transport.
	if req.URL.Scheme != "https" {
		return t.h1.RoundTrip(req)
	}

	host := req.URL.Hostname()
	port := req.URL.Port()
	if port == "" {
		port = "443"
	}
	addr := net.JoinHostPort(host, port)

	conn, proto, err := dialUTLS(addr, host)
	if err != nil {
		return nil, err
	}

	if proto == "h2" {
		return roundTripH2(req, conn)
	}

	// HTTP/1.1 fallback — close the pre-dialed conn and let h1 re-dial.
	// Rare for claude.ai which uses h2.
	conn.Close()
	return t.h1.RoundTrip(req)
}

func dialUTLS(addr, host string) (net.Conn, string, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, "", err
	}

	tlsConn := utls.UClient(conn, &utls.Config{
		ServerName: host,
	}, utls.HelloChrome_Auto)

	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, "", err
	}

	proto := tlsConn.ConnectionState().NegotiatedProtocol
	return tlsConn, proto, nil
}

func roundTripH2(req *http.Request, conn net.Conn) (*http.Response, error) {
	h2 := &http2.Transport{
		AllowHTTP:          false,
		DisableCompression: false,
	}

	h2conn, err := h2.NewClientConn(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("h2 client conn: %w", err)
	}

	return h2conn.RoundTrip(req)
}
