package diagnosis

import (
	"strings"
	"testing"
)

// languageRule이 알려진 언어 코드에는 지시문을, 빈 값/미지원 값에는 빈 문자열을 반환하는지 검증한다
// (Settings의 "응답 언어" 드롭다운이 실제 프롬프트에 반영되는 경로).
func TestLanguageRule(t *testing.T) {
	if r := languageRule(""); r != "" {
		t.Fatalf("empty language should produce no instruction, got %q", r)
	}
	if r := languageRule("xx"); r != "" {
		t.Fatalf("unknown language should produce no instruction, got %q", r)
	}
	for _, lang := range []string{"en", "ko", "zh", "la", "ja", "fr", "de"} {
		r := languageRule(lang)
		if r == "" {
			t.Fatalf("known language %q should produce an instruction", lang)
		}
		if name := languageNames[lang]; !strings.Contains(r, name) {
			t.Fatalf("instruction for %q should mention %q, got %q", lang, name, r)
		}
	}
}
