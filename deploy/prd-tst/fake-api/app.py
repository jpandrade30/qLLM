"""Stdlib-only crew API. Not the harness test-api (/users)."""
from http.server import BaseHTTPRequestHandler, HTTPServer
import json

DRIVERS = [
    {"id": "drv-1", "badge": "B-99", "full_name": "Kim Fleet"},
    {"id": "drv-2", "badge": "B-12", "full_name": "Alex Yard"},
]


class H(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def _json(self, code, obj):
        raw = json.dumps(obj).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        path = self.path.split("?", 1)[0]
        if path == "/health":
            self._json(200, {"ok": True, "system": "fleet-ops"})
            return
        if path == "/drivers":
            self._json(200, DRIVERS)
            return
        self._json(404, {"error": "not found"})


if __name__ == "__main__":
    HTTPServer(("0.0.0.0", 8080), H).serve_forever()
