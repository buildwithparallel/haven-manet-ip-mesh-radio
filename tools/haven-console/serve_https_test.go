package main

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestPlainHTTPRedirectsToHTTPS(t *testing.T) {
	cert, err := loadOrCreateCert(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "toolchain")
		}),
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}},
		ReadHeaderTimeout: 5 * time.Second,
	}
	go serveHTTPSListener(ln, srv)
	addr := ln.Addr().String()
	t.Cleanup(func() { _ = ln.Close() })

	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := noFollow.Get("http://" + addr + "/mesh")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if got, want := resp.Header.Get("Location"), "https://"+addr+"/mesh"; got != want {
		t.Fatalf("location %q, want %q", got, want)
	}

	httpsClient := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	resp, err = httpsClient.Get("https://" + addr + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) != "toolchain" {
		t.Fatalf("https response %d %q", resp.StatusCode, body)
	}
}
