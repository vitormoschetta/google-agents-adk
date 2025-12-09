package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/joho/godotenv"
	"github.com/vitormoschetta/go-adk/internal/handler"
	"github.com/vitormoschetta/go-adk/internal/server"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found or could not be loaded")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Criar servidor
	srv, err := server.NewServer(ctx)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Criar handlers
	h := handler.NewHandler(srv)

	// Configurar rotas com os handlers
	srv.SetupRouter(h.HandleRoot, h.HandleHealth, h.HandleChat, h.HandleTools)

	// Iniciar servidor
	srv.Start(ctx)
}
