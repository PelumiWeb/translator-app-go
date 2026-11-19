"""Serves the built site (web/out) as the host does: the files, and /v1/ passed
to Shinar's server, here the one on this Mac (web/vercel.json does the same on
Vercel). `make web-preview` runs it.

    python3 -I web/scripts/preview.py <out dir> <port> <server URL>
"""

import functools
import http.server
import sys
import urllib.error
import urllib.request

OUT, PORT, SERVER = sys.argv[1], int(sys.argv[2]), sys.argv[3].rstrip("/")


class Handler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        if self.path.startswith("/v1/"):
            self.pass_on()
        else:
            super().do_GET()

    def do_POST(self):
        if self.path.startswith("/v1/"):
            self.pass_on()
        else:
            self.send_error(405)

    def pass_on(self):
        length = int(self.headers.get("Content-Length") or 0)
        request = urllib.request.Request(
            SERVER + self.path,
            data=self.rfile.read(length) if length else None,
            method=self.command,
            headers={k: v for k, v in self.headers.items() if k.lower() in ("content-type", "authorization")},
        )
        try:
            with urllib.request.urlopen(request, timeout=10) as response:
                self.reply(response.status, response.headers.get("Content-Type"), response.read())
        except urllib.error.HTTPError as error:
            self.reply(error.code, error.headers.get("Content-Type"), error.read())
        except OSError:
            self.reply(502, "application/json", b'{"error":{"code":"unreachable","message":"the server on this Mac is not running"}}')

    def reply(self, status, content_type, body):
        self.send_response(status)
        if content_type:
            self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


handler = functools.partial(Handler, directory=OUT)
print(f"Serving {OUT} on http://localhost:{PORT}, with /v1/ passed to {SERVER}")
http.server.ThreadingHTTPServer(("127.0.0.1", PORT), handler).serve_forever()
