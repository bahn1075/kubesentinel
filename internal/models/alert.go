package models

import "time"

// AlertmanagerPayload는 Alertmanager webhook receiver가 전송하는 페이로드입니다.
// (Alertmanager webhook spec v4 기준, architecture.md §4.1)
type AlertmanagerPayload struct {
	Version      string  `json:"version"`
	GroupKey     string  `json:"groupKey"`
	Status       string  `json:"status"` // "firing" | "resolved"
	Receiver     string  `json:"receiver"`
	GroupLabels  Labels  `json:"groupLabels"`
	CommonLabels Labels  `json:"commonLabels"`
	Alerts       []Alert `json:"alerts"`
}

// Labels는 라벨/어노테이션 맵입니다.
type Labels map[string]string

// Alert는 개별 alert 항목입니다.
type Alert struct {
	Status       string    `json:"status"`
	Labels       Labels    `json:"labels"`
	Annotations  Labels    `json:"annotations"`
	StartsAt     time.Time `json:"startsAt"`
	EndsAt       time.Time `json:"endsAt"`
	GeneratorURL string    `json:"generatorURL"`
	Fingerprint  string    `json:"fingerprint"`
}
