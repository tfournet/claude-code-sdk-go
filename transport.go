package claudecode

import (
	"net"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
)

// newChromeTransport returns an http.Transport that mimics Chrome's TLS fingerprint.
// Cloudflare blocks Go's default crypto/tls fingerprint on claude.ai POST endpoints.
// Using utls with Chrome's ClientHello spec passes Cloudflare's bot detection.
func newChromeTransport() *http.Transport {
	return &http.Transport{
		DialTLS: func(network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
			}

			conn, err := net.DialTimeout(network, addr, 10*time.Second)
			if err != nil {
				return nil, err
			}

			tlsConn := utls.UClient(conn, &utls.Config{
				ServerName: host,
			}, utls.HelloChrome_Auto)

			if err := tlsConn.Handshake(); err != nil {
				conn.Close()
				return nil, err
			}

			return tlsConn, nil
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
}
