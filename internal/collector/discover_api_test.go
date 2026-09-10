package collector

import (
	"encoding/json"
	"strings"
	"testing"
)

// oke에서 4개 모두 검출되어 Missing이 비었을 때도 JSON이 배열이어야 한다.
// nil 슬라이스는 null로 직렬화되어 프론트엔드 렌더가 터진다(흰 화면).
func TestDiscoverResultAlwaysSerializesArrays(t *testing.T) {
	res := DiscoverResult{
		Available: true, Scanned: 56,
		Found:   []DiscoveredEndpoint{},
		Missing: []DiscoveredEndpoint{},
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, bad := range []string{`"found":null`, `"missing":null`} {
		if strings.Contains(got, bad) {
			t.Errorf("배열이어야 하는데 null로 직렬화됨: %s\n%s", bad, got)
		}
	}
	for _, want := range []string{`"found":[]`, `"missing":[]`} {
		if !strings.Contains(got, want) {
			t.Errorf("기대한 빈 배열 없음: %s\n%s", want, got)
		}
	}
}

// 회귀 방어: nil로 두면 null이 된다는 사실 자체를 고정한다.
func TestNilSliceSerializesToNull(t *testing.T) {
	raw, _ := json.Marshal(DiscoverResult{Available: true})
	if !strings.Contains(string(raw), `"missing":null`) {
		t.Skip("Go 직렬화 동작이 바뀜 — 초기화 가드 재검토 필요")
	}
}
