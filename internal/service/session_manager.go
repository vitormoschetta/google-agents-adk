package service

import (
	"sync"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/genai"
)

// ChatSession representa uma sessão de conversação HTTP
type ChatSession struct {
	ID      string
	Agent   agent.Agent
	History []*genai.Content
	Mu      sync.Mutex
}

// SessionManager gerencia sessões de conversação HTTP
type SessionManager struct {
	sessions map[string]*ChatSession
	mu       sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*ChatSession),
	}
}

// GetOrCreate obtém uma sessão existente ou cria uma nova
func (sm *SessionManager) GetOrCreate(sessionID string, a agent.Agent) *ChatSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sessionID == "" {
		sessionID = generateSessionID()
	}

	if chatSession, exists := sm.sessions[sessionID]; exists {
		return chatSession
	}

	chatSession := &ChatSession{
		ID:      sessionID,
		Agent:   a,
		History: []*genai.Content{},
	}
	sm.sessions[sessionID] = chatSession
	return chatSession
}

func generateSessionID() string {
	return time.Now().Format("20060102150405")
}
