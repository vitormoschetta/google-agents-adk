package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/genai"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
)

// ChatRequest representa a requisição para o endpoint de chat
type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
}

// ChatResponse representa a resposta do endpoint de chat
type ChatResponse struct {
	Response  string `json:"response"`
	SessionID string `json:"session_id"`
	Error     string `json:"error,omitempty"`
}

// ChatSession representa uma sessão de conversação HTTP
type ChatSession struct {
	ID      string
	Agent   agent.Agent
	History []*genai.Content
	mu      sync.Mutex
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

// Server representa o servidor HTTP com todas as dependências
type Server struct {
	agent          agent.Agent
	agentRunner    *runner.Runner
	sessionManager *SessionManager
	sessionService session.Service
	mcpEndpoint    string
	router         chi.Router
}

// NewServer cria uma nova instância do servidor
func NewServer(ctx context.Context) (*Server, error) {
	// Criar o modelo LLM
	llmModel, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}

	mcpEndpoint := os.Getenv("MCP_ENDPOINT")
	if mcpEndpoint == "" {
		return nil, fmt.Errorf("MCP_ENDPOINT is not set")
	}

	// Criar MCP transport
	transport := &mcp.StreamableClientTransport{
		Endpoint: mcpEndpoint,
	}

	mcpToolSet, err := mcptoolset.New(mcptoolset.Config{
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP tool set: %w", err)
	}

	// Criar LLMAgent com MCP tool set
	a, err := llmagent.New(llmagent.Config{
		Name:        "helper_agent",
		Model:       llmModel,
		Description: "Helper agent with MCP tools.",
		Instruction: "You are a helpful assistant that helps users with various tasks using MCP tools.",
		Toolsets: []tool.Toolset{
			mcpToolSet,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Criar session service (in-memory for HTTP server)
	sessionService := session.InMemoryService()

	// Criar runner para executar o agente
	agentRunner, err := runner.New(runner.Config{
		AppName:        "go-adk-http-server",
		Agent:          a,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	s := &Server{
		agent:          a,
		agentRunner:    agentRunner,
		sessionManager: NewSessionManager(),
		sessionService: sessionService,
		mcpEndpoint:    mcpEndpoint,
	}

	s.setupRouter()
	return s, nil
}

// setupRouter configura as rotas e middlewares do Chi
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Rotas
	r.Get("/", s.handleRoot)
	r.Get("/health", s.handleHealth)

	// API Routes
	r.Route("/api", func(r chi.Router) {
		r.Post("/chat", s.handleChat)
		r.Get("/tools", s.handleTools)
	})

	s.router = r
}

// handleRoot retorna informações sobre o serviço
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"service": "ADK Agent with MCP Tools",
		"endpoints": map[string]interface{}{
			"chat": map[string]interface{}{
				"url":         "http://localhost:8080/api/chat",
				"method":      "POST",
				"description": "Send a message to the agent",
				"example": map[string]string{
					"message":    "Hello, how can you help me?",
					"session_id": "optional-session-id",
				},
			},
			"health": map[string]interface{}{
				"url":         "http://localhost:8080/health",
				"method":      "GET",
				"description": "Health check endpoint",
			},
			"tools": map[string]interface{}{
				"url":         "http://localhost:8080/api/tools",
				"method":      "GET",
				"description": "List available MCP tools",
			},
		},
		"agent": map[string]string{
			"name":        "helper_agent",
			"description": "Helper agent with MCP tools",
		},
	}

	json.NewEncoder(w).Encode(response)
}

// handleHealth retorna o status de saúde do servidor
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleTools retorna informações sobre as ferramentas MCP disponíveis
func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"message":      "MCP tools are available through the agent",
		"note":         "To see available tools, ask the agent 'What tools do you have available?' in a chat message",
		"mcp_endpoint": s.mcpEndpoint,
	}

	json.NewEncoder(w).Encode(response)
}

// handleChat processa mensagens enviadas ao agente
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse do JSON
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		json.NewEncoder(w).Encode(ChatResponse{
			Error: "Invalid JSON format",
		})
		return
	}
	defer r.Body.Close()

	if req.Message == "" {
		json.NewEncoder(w).Encode(ChatResponse{
			Error: "Message is required",
		})
		return
	}

	// Obter ou criar sessão HTTP (para tracking local)
	chatSess := s.sessionManager.GetOrCreate(req.SessionID, s.agent)

	chatSess.mu.Lock()
	defer chatSess.mu.Unlock()

	// Executar o agente com a mensagem usando o runner
	log.Printf("Processing message in session %s: %s", chatSess.ID, req.Message)

	// Criar contexto de execução
	execCtx := context.Background()

	// Verificar se a sessão existe no SessionService do ADK, se não criar
	_, err := s.sessionService.Get(execCtx, &session.GetRequest{
		AppName:   "go-adk-http-server",
		SessionID: chatSess.ID,
	})
	if err != nil {
		// Sessão não existe, tentar criar nova
		_, createErr := s.sessionService.Create(execCtx, &session.CreateRequest{
			AppName:   "go-adk-http-server",
			SessionID: chatSess.ID,
			UserID:    "default-user",
		})
		if createErr != nil && !strings.Contains(createErr.Error(), "already exists") {
			// Erro real (não é "já existe")
			log.Printf("Error creating session in service: %v", createErr)
			json.NewEncoder(w).Encode(ChatResponse{
				Error:     fmt.Sprintf("Failed to create session: %v", createErr),
				SessionID: chatSess.ID,
			})
			return
		}
	}

	// Criar conteúdo do usuário
	userContent := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: req.Message},
		},
	}

	// Usar o runner para executar o agente com as ferramentas MCP
	var responseText strings.Builder
	var lastError error

	// O runner.Run executa o agente e retorna eventos de sessão
	for event, err := range s.agentRunner.Run(execCtx, "default-user", chatSess.ID, userContent, agent.RunConfig{}) {
		if err != nil {
			lastError = err
			log.Printf("Error running agent: %v", err)
			break
		}

		if event != nil && event.Content != nil {
			// Extrair texto de todas as partes do conteúdo
			if len(event.Content.Parts) > 0 {
				for _, part := range event.Content.Parts {
					if part.Text != "" {
						responseText.WriteString(part.Text)
					}
				}
			}
		}
	}

	if lastError != nil {
		json.NewEncoder(w).Encode(ChatResponse{
			Error:     fmt.Sprintf("Failed to process message: %v", lastError),
			SessionID: chatSess.ID,
		})
		return
	}

	responseStr := responseText.String()
	if responseStr == "" {
		responseStr = "O agente processou a mensagem, mas não retornou uma resposta."
	}

	log.Printf("Agent response in session %s: %s", chatSess.ID, responseStr)

	// Retornar a resposta
	json.NewEncoder(w).Encode(ChatResponse{
		Response:  responseStr,
		SessionID: chatSess.ID,
	})
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found or could not be loaded")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	startHTTPServer(ctx)
}

// startHTTPServer inicia o servidor HTTP com o ADK Agent
func startHTTPServer(ctx context.Context) {
	// Criar servidor
	srv, err := NewServer(ctx)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Configurar servidor HTTP com graceful shutdown
	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      srv.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Goroutine para iniciar o servidor
	go func() {
		log.Println("╔════════════════════════════════════════════════════╗")
		log.Println("║   ADK Agent HTTP Server com MCP Tools             ║")
		log.Println("╚════════════════════════════════════════════════════╝")
		log.Println("")
		log.Println("🚀 Servidor HTTP iniciado na porta :8080")
		log.Println("📦 Router: Chi v5")
		log.Println("")
		log.Println("📌 Endpoints disponíveis:")
		log.Println("   • Info:      http://localhost:8080/ (GET)")
		log.Println("   • Health:    http://localhost:8080/health (GET)")
		log.Println("   • Chat API:  http://localhost:8080/api/chat (POST)")
		log.Println("   • Tools:     http://localhost:8080/api/tools (GET)")
		log.Println("")
		log.Println("💡 Exemplo de uso com curl:")
		log.Println(`   curl -X POST http://localhost:8080/api/chat \`)
		log.Println(`        -H "Content-Type: application/json" \`)
		log.Println(`        -d '{"message":"Hello, what can you do?"}'`)
		log.Println("")
		log.Println("⚠️  Pressione Ctrl+C para parar o servidor")
		log.Println("")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Aguardar sinal de interrupção
	<-ctx.Done()
	log.Println("\n🛑 Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("❌ Server shutdown error: %v", err)
	}
	log.Println("✅ Server stopped gracefully")
}
