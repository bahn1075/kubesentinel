#!/usr/bin/env python3
"""로컬 테스트용 OpenAI 호환 mock LLM.

POST /v1/chat/completions 에 대해 KubeSentinel 진단 엔진이 기대하는
구조화된 RCA JSON(root_cause/summary/confidence/proposed_actions)을
choices[0].message.content 안의 JSON 문자열로 반환한다.
실제 LLM 없이 alert → 진단 → 알림 흐름을 끝까지 검증하는 용도.
"""
from http.server import BaseHTTPRequestHandler, HTTPServer
import json

# 진단 엔진이 content 문자열에서 추출/파싱하는 RCA 결과
RCA = {
    "root_cause": "memory limit(512Mi)이 워킹셋(peak ~730Mi) 대비 부족하여 OOMKilled 발생.",
    "summary": "최근 1시간 OOMKill 3회. 컨테이너 working set이 limit를 반복 초과. limit 상향 권장.",
    "confidence": 0.82,
    "proposed_actions": [
        {
            "type": "git_pr",
            "description": "memory limit 512Mi → 1Gi 상향",
            "target": "apps/demo/values.yaml",
            "risk": "medium",
        }
    ],
}


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        self.rfile.read(length)  # 요청 본문은 무시(mock)
        body = json.dumps(
            {"choices": [{"message": {"role": "assistant", "content": json.dumps(RCA)}}]}
        ).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
        print("[mock-llm] returned RCA for chat/completions", flush=True)

    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'{"status":"ok"}')

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    print("[mock-llm] listening on :8080", flush=True)
    HTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
