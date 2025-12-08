# 🤖 Go ADK + MCP - Agente IA

Aplicação Go que implementa um agente de IA usando **Google ADK (Agent Development Kit)** integrado com **MCP (Model Context Protocol)**, permitindo comunicação avançada com ferramentas e serviços externos.

## 🚀 Características

- ✅ **Google ADK** - Framework oficial do Google para desenvolvimento de agentes
- ✅ **Gemini 2.5 Flash** - Modelo de IA avançado e rápido do Google
- ✅ **MCP Integration** - Model Context Protocol para comunicação com ferramentas externas
- ✅ **Full Launcher** - Interface de linha de comando completa para interação
- ✅ **Environment Variables** - Configuração segura via variáveis de ambiente
- ✅ **Context Management** - Gerenciamento adequado de contexto e sinais de interrupção
- ✅ **Extensível** - Fácil adição de novos toolsets MCP

## 📋 Pré-requisitos

- Go 1.24.4 ou superior
- Chave de API do Google AI (Gemini)
- Endpoint MCP configurado (servidor MCP rodando)

## 🔑 Obter Credenciais

### API Key do Google AI

1. Acesse [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Crie uma nova API key
3. Copie a chave gerada

### MCP Endpoint

Configure um servidor MCP compatível. O endpoint deve ser acessível via URL (ex: `http://localhost:3000/mcp`)

## ⚙️ Instalação

### 1. Clone o repositório

```bash
git clone <seu-repositorio>
cd go-adk
```

### 2. Configure as variáveis de ambiente

Crie um arquivo `.env` baseado no `.env.example`:

```bash
cp .env.example .env
```

Edite o arquivo `.env` e configure suas credenciais:

```bash
# Google API Key para usar o Gemini
GOOGLE_API_KEY=sua_chave_api_aqui

# Endpoint do servidor MCP
MCP_ENDPOINT=http://localhost:3000/mcp

# GitHub PAT (opcional, se usar modo GitHub)
GITHUB_PAT=seu_github_token_aqui
```

### 3. Instale as dependências

```bash
go mod download
go mod tidy
```

### 4. Execute a aplicação

```bash
go run main.go
```

A aplicação iniciará em modo CLI interativo.

## 💬 Modo de Uso

A aplicação executa via **linha de comando** usando o **Full Launcher** do ADK. Ao rodar, você pode:

### Interação via CLI

```bash
# Executar modo interativo
go run main.go

# O agente aguardará suas mensagens no terminal
# Digite suas perguntas e pressione Enter
# Use Ctrl+C para sair
```

### Exemplo de Uso

```
$ go run main.go
> Como posso ajudá-lo?
Olá! Preciso de ajuda com...

> [Agente responde usando Gemini 2.5 Flash e ferramentas MCP]
```

## 🔧 Componentes Principais

### Agente LLM

```go
llmagent.New(llmagent.Config{
    Name:        "helper_agent",
    Model:       model,  // Gemini 2.5 Flash
    Description: "Helper agent.",
    Instruction: "You are a helpful assistant that helps users with various tasks.",
    Toolsets: []tool.Toolset{
        mcpToolSet,  // Ferramentas MCP
    },
})
```

### MCP Transport

O agente se conecta a um servidor MCP via endpoint configurado:

```go
transport := &mcp.StreamableClientTransport{
    Endpoint: mcpEndpoint,  // Do .env
}
```

### Modelo Gemini

Utiliza o **Gemini 2.5 Flash** para respostas rápidas e eficientes:

```go
model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
    APIKey: os.Getenv("GOOGLE_API_KEY"),
})
```

## 🛠️ Estrutura do Projeto

```
go-adk/
├── main.go           # Código principal da aplicação
├── go.mod            # Dependências do Go
├── go.sum            # Checksums das dependências
├── .env              # Variáveis de ambiente (não commitado - SEGURO)
├── .env.example      # Exemplo de configuração
├── .gitignore        # Arquivos ignorados pelo Git
├── .vscode/          # Configurações do VS Code (não commitado)
└── README.md         # Esta documentação
```

## 🔒 Segurança

O projeto está configurado corretamente para **não commitar dados sensíveis**:

- ✅ `.env` está no `.gitignore` (suas chaves API estão seguras)
- ✅ `.vscode/` está no `.gitignore` (configurações locais protegidas)
- ✅ O código usa `os.Getenv()` (nunca hardcoding de credenciais)
- ✅ `.env.example` contém apenas exemplos (sem valores reais)

**Você pode fazer commits com segurança!**

## 🔧 Personalização

### Modificar o Agente

Edite a configuração do agente em `main.go`:

```go
a, err := llmagent.New(llmagent.Config{
    Name:        "seu_agente_personalizado",
    Model:       model,
    Description: "Sua descrição personalizada",
    Instruction: "Suas instruções personalizadas para o agente",
    Toolsets: []tool.Toolset{
        mcpToolSet,
        // Adicione mais toolsets aqui
    },
})
```

### Trocar o Modelo Gemini

Para usar outro modelo do Gemini:

```go
model, err := gemini.NewModel(ctx, "gemini-2.0-pro", &genai.ClientConfig{
    APIKey: os.Getenv("GOOGLE_API_KEY"),
})
```

Modelos disponíveis:
- `gemini-2.5-flash` (rápido e eficiente)
- `gemini-2.0-pro` (mais avançado)
- `gemini-1.5-pro` (versão anterior)

### Adicionar Novos Toolsets MCP

```go
// Criar novo toolset MCP
customToolSet, err := mcptoolset.New(mcptoolset.Config{
    Transport: customTransport,
})

// Adicionar ao agente
Toolsets: []tool.Toolset{
    mcpToolSet,
    customToolSet,
}
```

## 📚 Recursos e Documentação

- [Google ADK Documentation](https://google.github.io/adk-docs/)
- [Google ADK GitHub](https://github.com/google/generative-ai-go/tree/main/adk)
- [Gemini API Documentation](https://ai.google.dev/docs)
- [Model Context Protocol (MCP)](https://modelcontextprotocol.io/)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [godotenv](https://github.com/joho/godotenv)

## 📦 Dependências Principais

```go
require (
    github.com/joho/godotenv v1.5.1                    // Carregamento de .env
    github.com/modelcontextprotocol/go-sdk v0.7.0      // SDK MCP
    google.golang.org/adk v0.2.0                        // Google ADK
    google.golang.org/genai v1.20.0                     // Gemini AI
)
```

## 🤝 Contribuindo

Contribuições são bem-vindas! Sinta-se livre para:

1. Fazer um Fork do projeto
2. Criar uma branch para sua feature (`git checkout -b feature/MinhaFeature`)
3. Commit suas mudanças (`git commit -m 'Adiciona MinhaFeature'`)
4. Push para a branch (`git push origin feature/MinhaFeature`)
5. Abrir um Pull Request

## 📝 Licença

Este projeto está sob a licença MIT.

## 🐛 Troubleshooting

### Erro: "MCP_ENDPOINT is not set"

Certifique-se de que o arquivo `.env` existe e contém a variável `MCP_ENDPOINT`:

```bash
MCP_ENDPOINT=http://localhost:3000/mcp
```

### Erro: "Failed to create model"

Verifique se a `GOOGLE_API_KEY` está correta no arquivo `.env`:

```bash
GOOGLE_API_KEY=sua_chave_valida_aqui
```

### Erro de conexão com MCP

Certifique-se de que o servidor MCP está rodando e acessível no endpoint configurado.

## 🚀 Próximos Passos

- [ ] Adicionar suporte a múltiplos toolsets MCP
- [ ] Implementar logging estruturado
- [ ] Adicionar testes unitários
- [ ] Criar API REST wrapper (opcional)
- [ ] Adicionar métricas e observabilidade

## 👨‍💻 Autor

Desenvolvido com ❤️ usando Go, Google ADK e Model Context Protocol.

---

**⭐ Se este projeto foi útil, considere dar uma estrela!**

