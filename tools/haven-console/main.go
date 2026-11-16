// Parallel Toolchain (haven-console): one page for a Haven mesh, served from the gate.
//
//	Performance  which node this phone is on, its route to the gate, signal,
//	       internet pings and a speed test
//	Mesh   every node, its HaLow links, and the live route from the gate
//	Talk   push-to-talk between phones, and a count track from the gate
//
// Every node runs the small agent in agent/haven-node.uc. This program runs
// on the gate only.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/json"
	"encoding/pem"
	"flag"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed web/*
var webFiles embed.FS

//go:embed count.ogg
var countTrack []byte

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

// loadOrCreateCert keeps a self-signed certificate in dir so phones only
// have to accept it once.
func loadOrCreateCert(dir string) (tls.Certificate, error) {
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if c, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		return c, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	host := hostname()
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "Parallel Toolchain " + host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{host, "localhost"},
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				tmpl.IPAddresses = append(tmpl.IPAddresses, ipn.IP)
			}
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return tls.Certificate{}, err
	}
	log.Printf("created self-signed certificate in %s", dir)
	return tls.LoadX509KeyPair(certPath, keyPath)
}

func main() {
	addr := flag.String("addr", ":8443", "HTTPS listen address")
	dir := flag.String("dir", "/etc/haven-console", "directory for the TLS certificate")
	agent := flag.String("agent", "http://127.0.0.1/cgi-bin/haven-node", "this node's agent URL")
	extra := flag.String("nodes", "", "comma-separated extra node addresses to check")
	countName := flag.String("count-name", "", "name the count track uses on the channel (default: <hostname> count)")
	flag.Parse()

	namesFile = filepath.Join(*dir, "names")
	cert, err := loadOrCreateCert(*dir)
	if err != nil {
		log.Fatalf("certificate: %v", err)
	}
	var extras []string
	for _, s := range strings.Split(*extra, ",") {
		if s = strings.TrimSpace(s); s != "" {
			extras = append(extras, s)
		}
	}
	if *countName == "" {
		*countName = hostname() + " count"
	}

	h := newHub()
	col := newCollector(*agent, extras)
	go col.run()
	rt := newRouter(col)
	go rt.run()
	count := newCountSource(h, *countName, countTrack)

	site, _ := fs.Sub(webFiles, "web")
	files := http.FileServer(http.FS(site))
	mux := http.NewServeMux()
	mux.HandleFunc("/events", h.events)
	mux.HandleFunc("/signal", h.signal)
	mux.HandleFunc("/api/field", col.fieldHandler)
	mux.HandleFunc("/api/mesh", col.meshHandler)
	mux.HandleFunc("/api/route", rt.handler)
	mux.HandleFunc("/api/ping", pingHandler)
	mux.HandleFunc("/api/speed/down", speedDown)
	mux.HandleFunc("/api/speed/up", speedUp)
	mux.HandleFunc("/api/count", count.handler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}},
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("Parallel Toolchain listening on https://%s", *addr)
	log.Fatal(srv.ListenAndServeTLS("", ""))
}
