package cli

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Run runs the interactive terminal loop.
func Run(ctx context.Context, model model.LLM) {
	// Initialize the ADK Agent with a custom system instruction
	a, err := llmagent.New(llmagent.Config{
		Name:        "dross_assistant",
		Model:       model,
		Description: "Your personal, extendable assistant running on your Mac via Ollama.",
		Instruction: "You are Dross, a helpful, intelligent personal assistant. Keep your answers clear, elegant, and concise.",
	})
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}

	// Initialize in-memory session service and runner
	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:           "dross",
		Agent:             a,
		SessionService:    sessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		log.Fatalf("failed to create runner: %v", err)
	}

	// Create the session
	userID := "console_user"
	resp, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: "dross",
		UserID:  userID,
	})
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}
	sessionID := resp.Session.ID()

	fmt.Println("====================================================")
	fmt.Println(" Dross Assistant - Local Gemma CLI (via Go ADK)")
	fmt.Println("====================================================")
	fmt.Println("Type your message and press Enter. Ctrl+C or EOF to exit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("User -> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		userMsg := genai.NewContentFromText(input, genai.RoleUser)
		fmt.Print("Dross -> ")

		prevText := ""
		// Use SSE streaming mode to get real-time dynamic incremental output
		err := func() error {
			for event, err := range r.Run(ctx, userID, sessionID, userMsg, agent.RunConfig{
				StreamingMode: agent.StreamingModeSSE,
			}) {
				if err != nil {
					return err
				}
				if event.LLMResponse.Content == nil {
					continue
				}

				text := ""
				for _, p := range event.LLMResponse.Content.Parts {
					text += p.Text
				}

				if !event.IsFinalResponse() {
					fmt.Print(text)
					prevText += text
				} else {
					if text != prevText {
						fmt.Print(text)
					}
					prevText = ""
				}
			}
			return nil
		}()

		if err != nil {
			fmt.Printf("\n[Error: %v]\n", err)
		}
		fmt.Println("\n")
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading input: %v", err)
	}
	fmt.Println("\nGoodbye!")
}
