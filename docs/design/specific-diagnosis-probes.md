# 설계: 구체적 진단 — 조사 프로브 + 조사 도구 (Specific Diagnosis)

## 배경 / 문제
현재 진단은 실패 **유형**(예: ImagePullBackOff)은 맞히지만, 조치는 룰북 수준의 일반론("이미지 존재/오타 확인, imagePullSecret 확인")에 머문다. 실제 근본 원인(예: `:latest`가 amd64 전용인데 노드는 arm64 → 아키텍처 불일치)을 짚지 못한다.

원인: LLM에게 **진짜 원인을 볼 도구가 없다.**
- 이미지 매니페스트(플랫폼) 조회 수단 없음 → arch 불일치를 볼 수 없음.
- 노드 아키텍처 노출 안 됨(`nodeHealth`는 비정상 노드만 보고).

## 목표
사람(SRE)이 직접 조사하듯, KubeSentinel이 **결정론적 조사(프로브)** + **LLM 조사 도구**로 실패 유형별 근본 원인을 **구체적으로** 짚고 조치를 제시한다. 작은 로컬 모델에서도 신뢰성 있게 동작하도록 상관(correlation)은 코드가 대신 수행한다.

## 접근 — 하이브리드
### A. 결정론적 프로브 (Enricher, 코드 주도 — 신뢰성)
룰 분류 카테고리에 따라 자동으로 심층 조사를 수행해 **구조화된 근거(`probe_findings`)**를 주입한다. LLM 성능과 무관하게 구체적 근거가 확보된다.

- **ImagePullBackOff**: 대상 이미지의 레지스트리 매니페스트 플랫폼 vs 클러스터 노드 아키텍처를 비교.
  - 태그 없음 → `이미지 태그 없음`
  - 플랫폼 ∩ 노드 arch = ∅ → `아키텍처 불일치: image=[amd64], nodes=[arm64] → 멀티아치 재빌드 또는 arch 일치 태그`
  - 교집합 있음 → arch 문제 아님(인증/네트워크/레이트리밋 가능성)
- (확장 여지) CrashLoopBackOff: 종료코드/직전로그, OOMKilled: limit vs 워킹셋, Unschedulable: taint/요청량. — v1은 ImagePull에 집중, 프레임만 일반화.

### B. LLM 조사 도구 확장 (agentic — 유연성)
agentic 루프(`analyzeAgentic`)에서 LLM이 직접 호출할 수 있는 read-only 도구 추가:
- `image_inspect(image)` — 레지스트리 매니페스트의 플랫폼·태그 존재 여부
- `k8s_get_nodes()` — 노드 arch·수·상태 요약

## 구현 요소
| 영역 | 파일 | 변경 |
|---|---|---|
| 레지스트리 조회 | `internal/collector/registry.go` (신규) | Docker Registry v2 최소 클라이언트(공개 docker.io). 매니페스트 리스트/단일 매니페스트에서 플랫폼 추출, 태그 존재 여부 |
| 노드/파드 조회 | `internal/collector/kube.go` | `NodeArchs()`, `NodeInfoSummary()`, `PodImages(ns,pod)` |
| 프로브 | `internal/collector/probe.go` (신규) | 카테고리별 프로브 디스패처. ImagePull arch/tag 진단 → `ProbeFindings` |
| 근거 모델 | `internal/models/evidence.go` | `ProbeFindings []string` 필드(LLM 컨텍스트 포함) |
| Enricher | `internal/collector/enrich.go` | 룰 분류 후 프로브 호출, 레지스트리 클라이언트 주입 |
| 도구 | `internal/collector/toolrunner.go` | `image_inspect`, `k8s_get_nodes` 등록 |
| 프롬프트 | `internal/diagnosis/engine.go` | `probe_findings`를 고신뢰 근거로 사용하라는 지침 |
| 뷰/프론트 | `views.go`, `types.ts`, `IncidentDetail.tsx` | `probeFindings`를 "자동 조사 결과"로 표시(LLM 실패 시에도 노출), evidenceQuality에 반영 |

## 리스크 / 완화
- **로컬 모델 추론력**: 프로브(A)가 상관을 코드로 수행 → 모델 의존 최소화.
- **레지스트리 접근**: 백엔드 아웃바운드 필요. v1은 **공개 docker.io만**. private/기타 레지스트리는 best-effort(에러 문자열 반환, 흐름 비차단).
- **RBAC**: nodes list/get·pods get은 기존 `nodeHealth`/`resourceStatus`에서 이미 사용 → 추가 권한 불필요.
- **지연**: 프로브는 카테고리 매칭 시에만 실행. 실패는 best-effort로 무시.

## 검증
- 단위 테스트: 이미지 ref 파싱, 프로브 비교 로직(arch 불일치/태그 없음/정상).
- 라이브: 백엔드에서 `image_inspect`/노드 arch 동작 확인. 합성 Alertmanager alert(arch 불일치 파드)로 E2E — 구체적 `probe_findings`와 진단 확인.
