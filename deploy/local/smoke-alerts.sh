#!/usr/bin/env bash
#
# 로컬 스모크 테스트 — alert를 "하나씩" 골라서 주입한다.
#
# 각 케이스는 internal/models/rules.go 의 ClassifyRules 가 서로 다른 카테고리로
# 분류하며, 그중 4개(OOMKilled/CrashLoopBackOff/ImagePullBackOff/Unschedulable)는
# internal/collector/probe.go 의 심층 프로브 분기까지 태운다.
# 주입 시점에는 in-cluster Events가 없어 분류는 alertname 기준으로 이뤄진다.
#
# 사전 준비:
#   kubectl -n kubesentinel port-forward svc/kubesentinel-kubesentinel-ai 8080:8080 &
#
# 사용법:
#   ./deploy/local/smoke-alerts.sh            # 목록에서 번호로 선택 (기본)
#   ./deploy/local/smoke-alerts.sh 3          # 3번 케이스만
#   ./deploy/local/smoke-alerts.sh OOMKilled  # 이름으로 지정
#   ./deploy/local/smoke-alerts.sh -l         # 목록만 출력
#   URL=http://localhost:9999 ./deploy/local/smoke-alerts.sh 1
#
# 한 번에 하나만 주입한다. 로컬 LLM은 동시 요청을 처리하지 못하는 경우가 많으므로
# 앞 건의 진단이 끝난 것을 로그/화면에서 확인한 뒤 다음 건을 주입하는 것을 권한다.
set -euo pipefail

URL="${URL:-http://localhost:8080}"

# alertname|namespace|workload|pod|severity|기대 분류|프로브|summary
CASES=(
  "OOMKilled|production|checkout|checkout-7d9f-abcde|critical|OOMKilled|O|container in pod checkout-7d9f-abcde was OOMKilled (limit 512Mi)"
  "KubePodCrashLooping|production|api-server|api-server-5c8b-x2qtp|critical|CrashLoopBackOff|O|pod api-server-5c8b-x2qtp is restarting 8 times in 10 minutes"
  "ImagePullBackOff|staging|notify-worker|notify-worker-64d7-ml9zk|warning|ImagePullBackOff|O|failed to pull image ghcr.io/acme/notify-worker:v2.1.0-arm64"
  "FailedScheduling|production|batch-indexer|batch-indexer-9f4c-t7rwd|warning|Unschedulable|O|0/3 nodes are available: insufficient memory"
  "CreateContainerConfigError|production|billing|billing-7b5d-qq83n|critical|ConfigError|-|container config error: secret billing-db not found"
  "KubeJobFailed|batch|nightly-rollup|nightly-rollup-29018-4kf2s|warning|JobFailed|-|job nightly-rollup reached backoff limit"
  "TargetDown|monitoring|node-exporter|node-exporter-vd82m|warning|TargetDown|-|30% of the node-exporter targets are down"
  "KubeNodeNotReady|kube-system|kubelet|node-worker-02|critical|NodeIssue|-|node node-worker-02 has been NotReady for 15 minutes"
  "KubePersistentVolumeFillingUp|production|postgres|postgres-0|warning|Unknown|-|PV claimed by postgres-0 is 92.4% full"
)

list_cases() {
  printf '\n  %-3s %-30s %-18s %s\n' "번호" "alertname" "기대 분류" "프로브"
  printf '  %s\n' "------------------------------------------------------------------------"
  local i=1
  for c in "${CASES[@]}"; do
    IFS='|' read -r alert _ _ _ _ want probe _ <<<"$c"
    printf '  %-3s %-30s %-18s %s\n' "$i" "$alert" "$want" "$probe"
    i=$((i + 1))
  done
  echo
}

# 인자를 번호 또는 alertname으로 해석해 0-based 인덱스를 반환한다.
resolve_index() {
  local sel="$1"
  if [[ "$sel" =~ ^[0-9]+$ ]]; then
    if [ "$sel" -ge 1 ] && [ "$sel" -le "${#CASES[@]}" ]; then
      echo $((sel - 1)); return 0
    fi
    return 1
  fi
  local i=0
  for c in "${CASES[@]}"; do
    if [ "${c%%|*}" = "$sel" ]; then echo "$i"; return 0; fi
    i=$((i + 1))
  done
  return 1
}

inject() {
  IFS='|' read -r alert ns workload pod sev want probe summary <<<"${CASES[$1]}"
  summary=${summary//\"/\\\"}

  echo
  echo "  주입: $alert"
  echo "  대상: $ns/$workload ($pod), severity=$sev"
  echo "  기대 분류: $want (심층 프로브: $probe)"

  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$URL/v1/alerts" \
    -H 'Content-Type: application/json' \
    -d "{\"receiver\":\"kubesentinel\",\"status\":\"firing\",\"alerts\":[{\"status\":\"firing\",
         \"labels\":{\"alertname\":\"$alert\",\"namespace\":\"$ns\",\"deployment\":\"$workload\",
         \"pod\":\"$pod\",\"severity\":\"$sev\"},
         \"annotations\":{\"summary\":\"$summary\"}}]}")

  echo "  → HTTP $code"
  if [ "$code" != "200" ]; then
    echo "  ⚠️  주입 실패. port-forward와 URL($URL)을 확인하세요." >&2
    return 1
  fi
  echo
  echo "  진단 진행 확인 (완료되면 ✅ Analysis Complete):"
  echo "    kubectl -n kubesentinel logs -l app.kubernetes.io/name=kubesentinel-ai -f"
  echo "  인시던트: inc-$(date +%Y%m%d)-$alert"
}

case "${1:-}" in
  -l|--list)
    list_cases; exit 0 ;;
  -h|--help)
    sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
esac

if [ $# -ge 1 ]; then
  if ! idx=$(resolve_index "$1"); then
    echo "알 수 없는 케이스: $1" >&2
    list_cases >&2
    exit 1
  fi
  inject "$idx"
  exit $?
fi

# 인자 없음 → 목록 보여주고 하나 선택
list_cases
if [ ! -t 0 ]; then
  echo "번호 또는 alertname을 인자로 주세요. 예: $0 1" >&2
  exit 1
fi
read -r -p "  주입할 번호 또는 alertname (취소: Enter): " sel
[ -z "$sel" ] && { echo "  취소했습니다."; exit 0; }
if ! idx=$(resolve_index "$sel"); then
  echo "  알 수 없는 케이스: $sel" >&2
  exit 1
fi
inject "$idx"
