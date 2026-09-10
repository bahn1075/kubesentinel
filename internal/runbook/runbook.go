// Package runbook은 운영자 제공 runbook(markdown+frontmatter)을 로드하고
// alertname/카테고리/키워드로 매칭한다. (architecture.md §4.3 Runbook — 메타데이터/키워드 검색, 벡터DB 불필요)
package runbook

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kubesentinel-ai/internal/models"
)

// Runbook은 frontmatter 메타데이터 + 본문(markdown)입니다.
type Runbook struct {
	Title    string
	Alerts   []string // 매칭할 alertname 목록
	Category string   // Rule Analyzer 카테고리와 매칭
	Keywords []string
	Body     string
}

// Store는 디렉토리에서 로드한 runbook 모음입니다.
type Store struct {
	books []Runbook
}

// Load는 dir의 *.md 파일을 파싱해 Store를 만든다. dir가 없거나 비면 빈 Store(매칭 시 nil).
func Load(dir string) *Store {
	s := &Store{}
	if strings.TrimSpace(dir) == "" {
		return s
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("[KubeSentinel] runbook dir not available (%s): %v\n", dir, err)
		return s
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		rb := parse(string(b))
		if rb.Title == "" {
			rb.Title = strings.TrimSuffix(e.Name(), ".md")
		}
		s.books = append(s.books, rb)
	}
	fmt.Printf("[KubeSentinel] loaded %d runbook(s) from %s\n", len(s.books), dir)
	return s
}

// Match는 alertname/category로 관련 runbook을 점수순으로 최대 max개 반환한다.
func (s *Store) Match(alertname, category string, max int) []models.RunbookMatch {
	if s == nil || len(s.books) == 0 {
		return nil
	}
	al := strings.ToLower(alertname)
	cat := strings.ToLower(category)
	type scored struct {
		rb    Runbook
		score int
	}
	var hits []scored
	for _, rb := range s.books {
		sc := 0
		for _, a := range rb.Alerts {
			if strings.EqualFold(a, alertname) {
				sc += 10
			}
		}
		if cat != "" && cat != "unknown" && strings.EqualFold(rb.Category, category) {
			sc += 8
		}
		for _, k := range rb.Keywords {
			lk := strings.ToLower(k)
			if lk != "" && (strings.Contains(al, lk) || strings.Contains(cat, lk)) {
				sc += 2
			}
		}
		if sc > 0 {
			hits = append(hits, scored{rb, sc})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if max > 0 && len(hits) > max {
		hits = hits[:max]
	}
	out := make([]models.RunbookMatch, 0, len(hits))
	for _, h := range hits {
		out = append(out, models.RunbookMatch{Title: h.rb.Title, Category: h.rb.Category, Body: h.rb.Body})
	}
	return out
}

// parse는 선택적 YAML-lite frontmatter(--- ... ---)와 본문을 분리·파싱한다(의존성 없는 최소 파서).
func parse(content string) Runbook {
	rb := Runbook{}
	body := content
	if strings.HasPrefix(content, "---") {
		rest := content[3:]
		if idx := strings.Index(rest, "\n---"); idx >= 0 {
			front := rest[:idx]
			body = strings.TrimLeft(rest[idx+4:], "\n")
			for _, line := range strings.Split(front, "\n") {
				line = strings.TrimSpace(line)
				k, v, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				k = strings.TrimSpace(strings.ToLower(k))
				v = strings.TrimSpace(v)
				switch k {
				case "title":
					rb.Title = strings.Trim(v, `"'`)
				case "category":
					rb.Category = strings.Trim(v, `"'`)
				case "alerts":
					rb.Alerts = parseList(v)
				case "keywords":
					rb.Keywords = parseList(v)
				}
			}
		}
	}
	rb.Body = strings.TrimSpace(body)
	return rb
}

// parseList는 "[a, b, c]" 또는 "a, b, c"를 문자열 슬라이스로 파싱한다.
func parseList(v string) []string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	var out []string
	for _, p := range strings.Split(v, ",") {
		p = strings.Trim(strings.TrimSpace(p), `"'`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
