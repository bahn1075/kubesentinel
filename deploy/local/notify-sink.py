#!/usr/bin/env python3
"""로컬 테스트용 알림 sink.

KubeSentinel notifier가 보내는 webhook(POST {"text"|"content": "..."})을 받아
본문을 stdout으로 출력한다. `docker compose logs notify-sink` 로 알림 내용을 확인한다.
실제 Discord/Slack/Teams 채널 없이 알림 단계를 검증하는 용도.
"""
from http.server import BaseHTTPRequestHandler, HTTPServer
import json


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length).decode("utf-8", "replace")
        try:
            data = json.loads(raw)
            msg = data.get("text") or data.get("content") or raw
        except Exception:
            msg = raw
        print("\n===== NOTIFICATION =====\n" + msg + "\n========================\n", flush=True)
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"ok")

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    print("[notify-sink] listening on :8080", flush=True)
    HTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
