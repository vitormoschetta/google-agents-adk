package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/vitormoschetta/go-adk/internal/model"
	"github.com/vitormoschetta/go-adk/internal/server"
)

// Handler contém as dependências necessárias para os handlers HTTP
type Handler struct {
	server *server.Server
}

// NewHandler cria uma nova instância do Handler
func NewHandler(srv *server.Server) *Handler {
	return &Handler{
		server: srv,
	}
}

// HandleRoot retorna informações sobre o serviço
func (h *Handler) HandleRoot(w http.ResponseWriter, r *http.Request) {
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

// HandleHealth retorna o status de saúde do servidor
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// HandleTools retorna informações sobre as ferramentas MCP disponíveis
func (h *Handler) HandleTools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"message":      "MCP tools are available through the agent",
		"note":         "To see available tools, ask the agent 'What tools do you have available?' in a chat message",
		"mcp_endpoint": h.server.McpEndpoint,
	}

	json.NewEncoder(w).Encode(response)
}

// HandleChat processa mensagens enviadas ao agente
func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse do JSON
	var req model.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		json.NewEncoder(w).Encode(model.ChatResponse{
			Error: "Invalid JSON format",
		})
		return
	}
	defer r.Body.Close()

	if req.Message == "" {
		json.NewEncoder(w).Encode(model.ChatResponse{
			Error: "Message is required",
		})
		return
	}

	// Obter ou criar sessão HTTP (para tracking local)
	chatSess := h.server.SessionManager.GetOrCreate(req.SessionID, h.server.Agent)

	chatSess.Mu.Lock()
	defer chatSess.Mu.Unlock()

	// Executar o agente com a mensagem usando o runner
	log.Printf("Processing message in session %s: %s", chatSess.ID, req.Message)

	// Criar contexto de execução
	execCtx := context.Background()

	// Verificar se a sessão existe no SessionService do ADK, se não criar
	_, err := h.server.SessionService.Get(execCtx, &session.GetRequest{
		AppName:   "go-adk-http-server",
		SessionID: chatSess.ID,
	})
	if err != nil {
		// Sessão não existe, tentar criar nova
		_, createErr := h.server.SessionService.Create(execCtx, &session.CreateRequest{
			AppName:   "go-adk-http-server",
			SessionID: chatSess.ID,
			UserID:    "default-user",
		})
		if createErr != nil && !strings.Contains(createErr.Error(), "already exists") {
			// Erro real (não é "já existe")
			log.Printf("Error creating session in service: %v", createErr)
			json.NewEncoder(w).Encode(model.ChatResponse{
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
	for event, err := range h.server.AgentRunner.Run(execCtx, "default-user", chatSess.ID, userContent, agent.RunConfig{}) {
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
		json.NewEncoder(w).Encode(model.ChatResponse{
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
	json.NewEncoder(w).Encode(model.ChatResponse{
		Response:  responseStr,
		SessionID: chatSess.ID,
	})
}
