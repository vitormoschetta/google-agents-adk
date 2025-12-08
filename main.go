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

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/genai"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
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

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found or could not be loaded")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Verificar se deve executar em modo HTTP ou CLI
	runHTTP := os.Getenv("RUN_HTTP_SERVER")
	if runHTTP == "true" {
		startHTTPServer(ctx)
	} else {
		startCLI(ctx)
	}
}

// startHTTPServer inicia o servidor HTTP com o ADK Agent
func startHTTPServer(ctx context.Context) {
	llmModel, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	mcpEndpoint := os.Getenv("MCP_ENDPOINT")
	if mcpEndpoint == "" {
		log.Fatalf("MCP_ENDPOINT is not set")
	}

	// Create MCP transport
	transport := &mcp.StreamableClientTransport{
		Endpoint: mcpEndpoint,
	}

	mcpToolSet, err := mcptoolset.New(mcptoolset.Config{
		Transport: transport,
	})
	if err != nil {
		log.Fatalf("Failed to create MCP tool set: %v", err)
	}

	// Create LLMAgent with MCP tool set
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
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Create session service (in-memory for HTTP server)
	sessionService := session.InMemoryService()

	// Create runner for executing the agent
	agentRunner, err := runner.New(runner.Config{
		AppName:        "go-adk-http-server",
		Agent:          a,
		SessionService: sessionService,
	})
	if err != nil {
		log.Fatalf("Failed to create runner: %v", err)
	}

	sessionManager := NewSessionManager()

	// Create a standard net/http ServeMux
	mux := http.NewServeMux()

	// Chat endpoint - envia uma mensagem para o agente
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

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
		chatSess := sessionManager.GetOrCreate(req.SessionID, a)

		chatSess.mu.Lock()
		defer chatSess.mu.Unlock()

		// Executar o agente com a mensagem usando o runner
		log.Printf("Processing message in session %s: %s", chatSess.ID, req.Message)

		// Criar contexto de execução
		execCtx := context.Background()

		// Verificar se a sessão existe no SessionService do ADK, se não criar
		_, err := sessionService.Get(execCtx, &session.GetRequest{
			AppName:   "go-adk-http-server",
			SessionID: chatSess.ID,
		})
		if err != nil {
			// Sessão não existe, tentar criar nova
			_, createErr := sessionService.Create(execCtx, &session.CreateRequest{
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
		for event, err := range agentRunner.Run(execCtx, "default-user", chatSess.ID, userContent, agent.RunConfig{}) {
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
	})

	// MCP Tools endpoint - lista as ferramentas disponíveis
	mux.HandleFunc("/api/tools", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":      "MCP tools are available through the agent",
			"note":         "To see available tools, ask the agent 'What tools do you have available?' in a chat message",
			"mcp_endpoint": mcpEndpoint,
		})
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	// Root endpoint com informações do serviço
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := `{
  "service": "ADK Agent with MCP Tools",
  "endpoints": {
    "chat": {
      "url": "http://localhost:8080/api/chat",
      "method": "POST",
      "description": "Send a message to the agent",
      "example": {
        "message": "Hello, how can you help me?",
        "session_id": "optional-session-id"
      }
    },
    "health": {
      "url": "http://localhost:8080/health",
      "method": "GET",
      "description": "Health check endpoint"
    }
  },
  "agent": {
    "name": "helper_agent",
    "description": "Helper agent with MCP tools"
  }
}`
		if _, err := w.Write([]byte(response)); err != nil {
			log.Printf("Failed to write response: %v", err)
		}
	})

	// Configurar servidor com graceful shutdown
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Goroutine para iniciar o servidor
	go func() {
		log.Println("╔════════════════════════════════════════════════════╗")
		log.Println("║   ADK Agent HTTP Server com MCP Tools             ║")
		log.Println("╚════════════════════════════════════════════════════╝")
		log.Println("")
		log.Println("🚀 Servidor HTTP iniciado na porta :8080")
		log.Println("")
		log.Println("📌 Endpoints disponíveis:")
		log.Println("   • Chat API:  http://localhost:8080/api/chat (POST)")
		log.Println("   • Health:    http://localhost:8080/health")
		log.Println("   • Info:      http://localhost:8080/")
		log.Println("")
		log.Println("💡 Exemplo de uso com curl:")
		log.Println(`   curl -X POST http://localhost:8080/api/chat \`)
		log.Println(`        -H "Content-Type: application/json" \`)
		log.Println(`        -d '{"message":"Hello, what can you do?"}'`)
		log.Println("")
		log.Println("⚠️  Pressione Ctrl+C para parar o servidor")
		log.Println("")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Aguardar sinal de interrupção
	<-ctx.Done()
	log.Println("\n🛑 Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("❌ Server shutdown error: %v", err)
	}
	log.Println("✅ Server stopped gracefully")
}

// startCLI inicia o modo CLI original
func startCLI(ctx context.Context) {
	model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	mcpEndpoint := os.Getenv("MCP_ENDPOINT")
	if mcpEndpoint == "" {
		log.Fatalf("MCP_ENDPOINT is not set")
	}

	// Create MCP transport
	transport := &mcp.StreamableClientTransport{
		Endpoint: mcpEndpoint,
	}

	mcpToolSet, err := mcptoolset.New(mcptoolset.Config{
		Transport: transport,
	})
	if err != nil {
		log.Fatalf("Failed to create MCP tool set: %v", err)
	}

	// Create LLMAgent with MCP tool set
	a, err := llmagent.New(llmagent.Config{
		Name:        "helper_agent",
		Model:       model,
		Description: "Helper agent.",
		Instruction: "You are a helpful assistant that helps users with various tasks.",
		Toolsets: []tool.Toolset{
			mcpToolSet,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(a),
	}
	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
