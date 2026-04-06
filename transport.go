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

// newChromeH1Transport returns an http.Transport that uses utls Chrome
// fingerprinting but forces HTTP/1.1 (no h2 ALPN). Required for WebSocket
// connections which don't work over HTTP/2.
func newChromeH1Transport() *http.Transport {
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
			// Use Chrome fingerprint but override NextProtos to exclude h2.
			// This makes the server negotiate HTTP/1.1 for WebSocket upgrades.
			tlsConn := utls.UClient(conn, &utls.Config{
				ServerName: host,
				NextProtos: []string{"http/1.1"},
			}, utls.HelloCustom)
			// Apply Chrome-like spec manually with http/1.1 only ALPN.
			spec := &utls.ClientHelloSpec{
				TLSVersMax: utls.VersionTLS13,
				TLSVersMin: utls.VersionTLS12,
				CipherSuites: []uint16{
					utls.GREASE_PLACEHOLDER,
					utls.TLS_AES_128_GCM_SHA256,
					utls.TLS_AES_256_GCM_SHA384,
					utls.TLS_CHACHA20_POLY1305_SHA256,
					utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					utls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
					utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					utls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
					utls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
					utls.TLS_RSA_WITH_AES_128_GCM_SHA256,
					utls.TLS_RSA_WITH_AES_256_GCM_SHA384,
					utls.TLS_RSA_WITH_AES_128_CBC_SHA,
					utls.TLS_RSA_WITH_AES_256_CBC_SHA,
				},
				Extensions: []utls.TLSExtension{
					&utls.SNIExtension{},
					&utls.SupportedCurvesExtension{Curves: []utls.CurveID{utls.X25519, utls.CurveP256, utls.CurveP384}},
					&utls.SupportedPointsExtension{SupportedPoints: []byte{0}},
					&utls.SessionTicketExtension{},
					&utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}},
					&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
						utls.ECDSAWithP256AndSHA256, utls.PSSWithSHA256,
						utls.PKCS1WithSHA256, utls.ECDSAWithP384AndSHA384,
						utls.PSSWithSHA384, utls.PKCS1WithSHA384,
						utls.PSSWithSHA512, utls.PKCS1WithSHA512,
					}},
					&utls.SupportedVersionsExtension{Versions: []uint16{utls.VersionTLS13, utls.VersionTLS12}},
					&utls.KeyShareExtension{KeyShares: []utls.KeyShare{{Group: utls.X25519}}},
				},
			}
			if err := tlsConn.ApplyPreset(spec); err != nil {
				conn.Close()
				return nil, fmt.Errorf("apply h1 spec: %w", err)
			}
			if err := tlsConn.Handshake(); err != nil {
				conn.Close()
				return nil, err
			}
			return tlsConn, nil
		},
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
	}
}
