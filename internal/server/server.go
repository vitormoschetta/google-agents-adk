package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"
	"google.golang.org/genai"

	"github.com/vitormoschetta/go-adk/internal/service"
)

// AuthenticatedTransport adiciona headers de autenticação às requisições HTTP
type AuthenticatedTransport struct {
	Base  http.RoundTripper
	Token string
}

func (t *AuthenticatedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clonar a requisição para não modificar a original
	reqCopy := req.Clone(req.Context())

	// Adicionar o header de autenticação
	if t.Token != "" {
		reqCopy.Header.Set("X-Tiger-Token", t.Token)
	}

	// Log da requisição (útil para debug)
	log.Printf("MCP Request: %s %s (with X-Tiger-Token)", reqCopy.Method, reqCopy.URL)

	// Executar a requisição com o transport base
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(reqCopy)
}

// Server representa o servidor HTTP com todas as dependências
type Server struct {
	Agent          agent.Agent
	AgentRunner    *runner.Runner
	SessionManager *service.SessionManager
	SessionService session.Service
	McpEndpoint    string
	Router         chi.Router
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

	// Obter o token de autenticação
	tigerToken := os.Getenv("X_TIGER_TOKEN")
	if tigerToken == "" {
		log.Println("Warning: X_TIGER_TOKEN is not set - MCP requests may fail with 403")
	}

	// Criar HTTP client com autenticação customizada
	httpClient := &http.Client{
		Transport: &AuthenticatedTransport{
			Base:  http.DefaultTransport,
			Token: tigerToken,
		},
		Timeout: 30 * time.Second,
	}

	// Criar MCP transport com o HTTP client autenticado
	transport := &mcp.StreamableClientTransport{
		Endpoint:   mcpEndpoint,
		HTTPClient: httpClient,
	}

	log.Printf("🔌 Connecting to MCP endpoint: %s", mcpEndpoint)

	mcpToolSet, err := mcptoolset.New(mcptoolset.Config{
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP tool set: %w", err)
	}

	log.Printf("✅ MCP toolset initialized successfully")

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
		Agent:          a,
		AgentRunner:    agentRunner,
		SessionManager: service.NewSessionManager(),
		SessionService: sessionService,
		McpEndpoint:    mcpEndpoint,
	}

	return s, nil
}

// SetupRouter configura as rotas e middlewares do Chi
func (s *Server) SetupRouter(
	handleRoot func(http.ResponseWriter, *http.Request),
	handleHealth func(http.ResponseWriter, *http.Request),
	handleChat func(http.ResponseWriter, *http.Request),
	handleTools func(http.ResponseWriter, *http.Request),
) {
	r := chi.NewRouter()

	// Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Rotas
	r.Get("/", handleRoot)
	r.Get("/health", handleHealth)

	// API Routes
	r.Route("/api", func(r chi.Router) {
		r.Post("/chat", handleChat)
		r.Get("/tools", handleTools)
	})

	s.Router = r
}

// Start inicia o servidor HTTP com graceful shutdown
func (s *Server) Start(ctx context.Context) {
	// Configurar servidor HTTP com graceful shutdown
	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      s.Router,
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
