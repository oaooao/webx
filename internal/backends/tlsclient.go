// Package backends provides shared HTTP transport utilities for webx adapters.
// tlsclient.go exposes a shared HTTP client that uses a Chrome TLS fingerprint
// via uTLS to avoid fingerprint-based bot detection.
package backends

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	utls "github.com/refraction-networking/utls"
)

// utlsDialer dials a TCP connection and upgrades it to TLS using uTLS with a
// Chrome ClientHello fingerprint.
//
// HTTP/2 note: HelloChrome_Auto advertises "h2" in ALPN, so servers may
// negotiate HTTP/2. However, http.Transport's DialTLSContext cannot drive HTTP/2
// framing on its own – that only works through the built-in HTTP/2 upgrade path.
// To stay safe we restrict ALPN to HTTP/1.1. For sites that require HTTP/2 the
// transport falls back transparently via a standard tls.Dial retry path.
type utlsDialer struct {
	dialer *net.Dialer
}

func (d *utlsDialer) dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	tcpConn, err := d.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	config := &utls.Config{
		ServerName: host,
		// Restrict ALPN to http/1.1 only. Chrome_Auto normally includes "h2"
		// but http.Transport cannot process HTTP/2 frames from a custom DialTLS
		// conn because it bypasses the internal http2.Transport upgrade.
		NextProtos: []string{"http/1.1"},
	}

	uconn := utls.UClient(tcpConn, config, utls.HelloChrome_Auto)
	if err := uconn.HandshakeContext(ctx); err != nil {
		_ = tcpConn.Close()
		// If the handshake fails (some servers reject HTTP/1.1-only ALPN),
		// fall back to a plain TLS connection without Chrome fingerprint.
		return stdTLSDial(ctx, d.dialer, network, addr, host)
	}

	// Double-check: if server somehow negotiated h2, fall back to plain TLS.
	if uconn.ConnectionState().NegotiatedProtocol == "h2" {
		_ = uconn.Close()
		return stdTLSDial(ctx, d.dialer, network, addr, host)
	}

	return uconn, nil
}

// stdTLSDial performs a plain crypto/tls dial as a fallback. This loses the
// Chrome fingerprint but keeps the request working.
func stdTLSDial(ctx context.Context, dialer *net.Dialer, network, addr, host string) (net.Conn, error) {
	tcpConn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(tcpConn, &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = tcpConn.Close()
		return nil, err
	}
	return tlsConn, nil
}

// NewUTLSClient returns an *http.Client that presents a Chrome TLS fingerprint.
// Falls back to standard TLS when HTTP/2 negotiation is detected.
func NewUTLSClient() *http.Client {
	d := &utlsDialer{
		dialer: &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}

	transport := &http.Transport{
		DialTLSContext:  d.dialTLS,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		// ForceAttemptHTTP2 must stay false: we drive the TLS layer ourselves
		// via DialTLSContext, so http.Transport's built-in HTTP/2 upgrade won't
		// fire, but we already guard against h2 in dialTLS above.
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
}

// sharedUTLSClient is the package-level default client used by FetchHTML.
// Initialised once; safe for concurrent reads after init.
var sharedUTLSClient = NewUTLSClient()
