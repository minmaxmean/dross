package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"minmax.uk/dross/cli"
	"minmax.uk/dross/ollama"
	"minmax.uk/dross/telegram"
)

func main() {
	// Parse execution mode flag
	modeFlag := flag.String("mode", "cli", "Execution mode: 'cli' or 'telegram'")
	flag.Parse()

	// Load environment variables from .env file (if present)
	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not loaded (using system environment variables if set)")
	}

	// Set up cancellation context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize local Ollama model (defaulting to gemma4:e2b)
	model, err := ollama.NewModel("gemma4:e2b")
	if err != nil {
		log.Fatalf("failed to create Ollama model: %v", err)
	}

	switch *modeFlag {
	case "cli":
		cli.Run(ctx, model)
	case "telegram":
		telegram.Run(ctx, model)
	default:
		log.Fatalf("invalid execution mode: %q. Supported modes: 'cli', 'telegram'", *modeFlag)
	}
}
