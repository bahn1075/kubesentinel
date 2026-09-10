package models

import "time"

// IgnoreRule은 사용자 관리 무시 규칙이다. keyword가 alert명 또는 대상에 부분일치하면
// 해당 alert를 인시던트로 처리하지 않는다(양쪽 와일드카드 = 부분문자열, 대소문자 무시).
type IgnoreRule struct {
	ID        int64     `json:"id"`
	Keyword   string    `json:"keyword"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
}
