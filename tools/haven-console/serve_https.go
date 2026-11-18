package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// serveHTTPS serves the Toolchain over TLS. A plain HTTP request to the same
// port is redirected to the https URL, so http://<gate>:8443 reaches the page.
func serveHTTPS(addr string, srv *http.Server) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return serveHTTPSListener(ln, srv)
}

func serveHTTPSListener(ln net.Listener, srv *http.Server) error {
	defer ln.Close()
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		go dispatchConn(c, srv)
	}
}

func dispatchConn(c net.Conn, srv *http.Server) {
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	var one [1]byte
	n, err := c.Read(one[:])
	_ = c.SetReadDeadline(time.Time{})
	if n != 1 || err != nil {
		c.Close()
		return
	}
	pc := &prefixConn{Conn: c, rest: one[:1]}
	if one[0] == 0x16 { // TLS handshake record
		_ = srv.Serve(&oneShotListener{conn: tls.Server(pc, srv.TLSConfig)})
		return
	}
	plain := &http.Server{
		Handler:           http.HandlerFunc(redirectToHTTPS),
		ReadHeaderTimeout: srv.ReadHeaderTimeout,
	}
	_ = plain.Serve(&oneShotListener{conn: pc})
}

func redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		host = "localhost"
	}
	http.Redirect(w, r, "https://"+host+r.URL.RequestURI(), http.StatusTemporaryRedirect)
}

// prefixConn returns bytes already read, then the rest of the connection.
type prefixConn struct {
	net.Conn
	rest []byte
}

func (c *prefixConn) Read(p []byte) (int, error) {
	if len(c.rest) > 0 {
		n := copy(p, c.rest)
		c.rest = c.rest[n:]
		return n, nil
	}
	return c.Conn.Read(p)
}

// oneShotListener hands its connection to http.Server once. Close does not
// close that connection; the server closes it when the request finishes.
type oneShotListener struct {
	conn net.Conn
	used bool
}

func (l *oneShotListener) Accept() (net.Conn, error) {
	if l.used {
		return nil, net.ErrClosed
	}
	l.used = true
	return l.conn, nil
}

func (l *oneShotListener) Close() error { return nil }

func (l *oneShotListener) Addr() net.Addr { return l.conn.LocalAddr() }
