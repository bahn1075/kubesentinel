package models

// AIClient는 LLM과 통신하는 인터페이스입니다. (architecture.md §4.2)
// provider 패키지의 AIGateway가 이 인터페이스를 구현하며,
// diagnosis 엔진은 구체 구현이 아닌 이 인터페이스에만 의존합니다.
type AIClient interface {
	// Chat은 단발 프롬프트를 보낸다(system + user).
	Chat(prompt string, context string) (*ChatResponse, error)
	// ChatMessages는 다중 턴 대화를 보낸다(agentic 루프·검증 패스용).
	ChatMessages(messages []ChatMessage) (*ChatResponse, error)
}

// ChatMessage는 OpenAI 호환 대화 메시지입니다.
type ChatMessage struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// ChatResponse는 AI의 응답을 담는 구조체입니다.
type ChatResponse struct {
	Content string `json:"content"`
}
