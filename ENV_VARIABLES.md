# Variáveis de Ambiente

Este projeto requer as seguintes variáveis de ambiente configuradas no arquivo `.env`:

## GOOGLE_API_KEY
**Obrigatório**

Chave de API do Google AI para usar o modelo Gemini.

- **Como obter**: Acesse [Google AI Studio](https://aistudio.google.com/app/apikey)
- **Exemplo**: `GOOGLE_API_KEY=AIzaSyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX`

## MCP_ENDPOINT
**Obrigatório**

URL do servidor MCP (Model Context Protocol) que fornece as ferramentas para o agente.

- **Formato**: URL completa incluindo protocolo e caminho
- **Exemplo**: `MCP_ENDPOINT=http://localhost:3000/mcp`

## X_TIGER_TOKEN
**Obrigatório para autenticação MCP**

Token de autenticação que será enviado no header `X-Tiger-Token` para todas as requisições ao MCP endpoint.

- **Uso**: Autenticação com o servidor MCP
- **Exemplo**: `X_TIGER_TOKEN=seu-token-secreto-aqui`
- **Nota**: Se não configurado, as requisições ao MCP podem falhar com erro `403 Forbidden`

## Exemplo de arquivo .env

```bash
# Google AI API Key
GOOGLE_API_KEY=AIzaSyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

# MCP Endpoint
MCP_ENDPOINT=http://localhost:3000/mcp

# X-Tiger-Token para autenticação MCP
X_TIGER_TOKEN=seu-token-secreto-aqui
```

## Configuração

1. Copie o conteúdo acima para um arquivo `.env` na raiz do projeto
2. Substitua os valores de exemplo pelos seus valores reais
3. Nunca commite o arquivo `.env` no git (já está no `.gitignore`)

