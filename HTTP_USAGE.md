# 🌐 Guia de Uso do Servidor HTTP

Este documento fornece exemplos práticos de como usar o servidor HTTP do ADK Agent.

## 🚀 Iniciando o Servidor

### Opção 1: Executar diretamente

```bash
go run main.go
```

### Opção 2: Compilar e Executar

```bash
# Compilar
go build -o adk-agent main.go

# Executar
./adk-agent
```

## 📡 Testando os Endpoints

### 1. Health Check

Verifica se o servidor está rodando:

```bash
curl http://localhost:8080/health
```

**Resposta esperada:** `OK`

### 2. Informações do Serviço

Obtém informações sobre os endpoints disponíveis:

```bash
curl http://localhost:8080/
```

**Resposta:**
```json
{
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
}
```

### 3. Enviar Mensagem (sem sessão)

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello, what can you do?"}'
```

**Resposta:**
```json
{
  "response": "Mensagem recebida: Hello, what can you do?. A integração completa com o agente requer ajustes na API de execução.",
  "session_id": "20251208143022"
}
```

### 4. Enviar Mensagem (com sessão)

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Continue our conversation",
    "session_id": "20251208143022"
  }'
```

## 💡 Exemplos Práticos

### Exemplo 1: Conversa Simples

```bash
# Primeira mensagem
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Olá! Como você pode me ajudar?"}'
```

### Exemplo 2: Manter Contexto de Conversação

```bash
# Passo 1: Apresentação
RESPONSE=$(curl -s -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Meu nome é Maria"}')

# Extrair session_id da resposta
SESSION_ID=$(echo $RESPONSE | jq -r '.session_id')

# Passo 2: Continuar a conversa com a mesma sessão
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d "{\"message\": \"Qual é o meu nome?\", \"session_id\": \"$SESSION_ID\"}"
```

### Exemplo 3: Usando Python

```python
import requests
import json

# Configuração
BASE_URL = "http://localhost:8080"

# Função para enviar mensagem
def send_message(message, session_id=None):
    payload = {"message": message}
    if session_id:
        payload["session_id"] = session_id
    
    response = requests.post(
        f"{BASE_URL}/api/chat",
        headers={"Content-Type": "application/json"},
        data=json.dumps(payload)
    )
    
    return response.json()

# Exemplo de uso
# Primeira mensagem
response1 = send_message("Olá! Meu nome é João")
print(f"Resposta: {response1['response']}")
session_id = response1['session_id']
print(f"Session ID: {session_id}")

# Segunda mensagem mantendo a sessão
response2 = send_message("Qual é o meu nome?", session_id)
print(f"Resposta: {response2['response']}")
```

### Exemplo 4: Usando JavaScript (Node.js)

```javascript
const axios = require('axios');

const BASE_URL = 'http://localhost:8080';

// Função para enviar mensagem
async function sendMessage(message, sessionId = null) {
    const payload = { message };
    if (sessionId) {
        payload.session_id = sessionId;
    }
    
    try {
        const response = await axios.post(
            `${BASE_URL}/api/chat`,
            payload,
            {
                headers: { 'Content-Type': 'application/json' }
            }
        );
        
        return response.data;
    } catch (error) {
        console.error('Error:', error.response?.data || error.message);
        throw error;
    }
}

// Exemplo de uso
(async () => {
    // Primeira mensagem
    const response1 = await sendMessage('Olá! Meu nome é João');
    console.log('Resposta:', response1.response);
    console.log('Session ID:', response1.session_id);
    
    // Segunda mensagem mantendo a sessão
    const response2 = await sendMessage(
        'Qual é o meu nome?',
        response1.session_id
    );
    console.log('Resposta:', response2.response);
})();
```

### Exemplo 5: Usando Postman

**Request:**
- **Método:** POST
- **URL:** `http://localhost:8080/api/chat`
- **Headers:**
  - `Content-Type: application/json`
- **Body (raw JSON):**
```json
{
  "message": "Como você pode me ajudar?",
  "session_id": "optional-session-id"
}
```

## 🔧 Tratamento de Erros

### Método HTTP Inválido

```bash
curl -X GET http://localhost:8080/api/chat
```

**Resposta:** `405 Method Not Allowed`

### JSON Inválido

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d 'mensagem inválida'
```

**Resposta:**
```json
{
  "response": "",
  "session_id": "",
  "error": "Invalid JSON format"
}
```

### Mensagem Vazia

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": ""}'
```

**Resposta:**
```json
{
  "response": "",
  "session_id": "",
  "error": "Message is required"
}
```

## 🛑 Parando o Servidor

Para parar o servidor gracefully, pressione `Ctrl+C` no terminal onde ele está executando.

O servidor executará um **graceful shutdown**, finalizando requisições em andamento antes de encerrar (timeout de 5 segundos).

## 📊 Monitoramento

### Verificar se o servidor está rodando

```bash
# Health check básico
curl -f http://localhost:8080/health && echo "Servidor OK" || echo "Servidor OFFLINE"
```

### Script de monitoramento (Bash)

```bash
#!/bin/bash
while true; do
    if curl -f -s http://localhost:8080/health > /dev/null; then
        echo "$(date): Servidor OK"
    else
        echo "$(date): Servidor OFFLINE"
    fi
    sleep 30
done
```

## 🔐 Próximos Passos (Segurança)

**⚠️ Importante:** Este servidor é para desenvolvimento/testes locais.

Para uso em produção, considere adicionar:

- ✅ Autenticação (JWT, API Keys)
- ✅ HTTPS/TLS
- ✅ Rate Limiting
- ✅ CORS configurado adequadamente
- ✅ Validação e sanitização de entrada
- ✅ Logging estruturado
- ✅ Métricas e observabilidade

## 📚 Recursos Adicionais

- [README.md](./README.md) - Documentação principal
- [Google ADK Documentation](https://google.github.io/adk-docs/)
- [Model Context Protocol](https://modelcontextprotocol.io/)

---

**Desenvolvido com ❤️ usando Go + Google ADK + MCP**

