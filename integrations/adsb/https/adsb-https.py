#!/usr/bin/env python3
"""Serve the dump1090 SkyAware map over HTTPS (TLS 1.3 via OpenSSL).

uhttpd's mbedtls build only speaks TLS 1.2 and stalls on some Android Chrome
handshakes, so this small server handles HTTPS for the map instead. Plain HTTP
stays on uhttpd (:8080). Each connection's TLS handshake runs in its own
thread with a timeout, so one slow client can't block the others.
"""
import functools
import http.server
import ssl

ROOT = "/usr/share/dump1090"
PORT = 8443
CERT, KEY = "/etc/adsb.crt", "/etc/adsb.key"

ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
ctx.minimum_version = ssl.TLSVersion.TLSv1_2
ctx.load_cert_chain(CERT, KEY)


class Handler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        # Never let a browser reuse a stale copy of the page or the data.
        self.send_header("Cache-Control", "no-cache")
        super().end_headers()

    def log_message(self, *args):
        pass


class Server(http.server.ThreadingHTTPServer):
    daemon_threads = True

    def finish_request(self, request, client_address):
        request.settimeout(15)
        try:
            tls = ctx.wrap_socket(request, server_side=True)
        except (ssl.SSLError, OSError):
            request.close()
            return
        self.RequestHandlerClass(tls, client_address, self)


Server(("", PORT), functools.partial(Handler, directory=ROOT)).serve_forever()
