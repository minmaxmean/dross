package telegram

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

type telegramHandler struct {
	runner       *runner.Runner
	allowedUsers string
}

// Run starts the Telegram Bot polling loop.
func Run(ctx context.Context, model model.LLM) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set in environment or .env file")
	}

	allowedUsers := os.Getenv("TELEGRAM_ALLOWED_USERS")
	if allowedUsers == "" {
		log.Println("WARNING: TELEGRAM_ALLOWED_USERS is empty. Nobody will be able to talk to the bot.")
	}

	handler, err := NewHandler(model, allowedUsers)
	if err != nil {
		log.Fatalf("failed to create telegram handler: %v", err)
	}

	// 3. Initialize Telegram Bot client
	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		log.Fatalf("failed to initialize telegram bot: %v", err)
	}

	log.Println("Telegram bot initialized successfully. Starting polling loop...")
	b.Start(ctx)
}

// NewHandler creates and configures the ADK Agent, session runner, and returns the Telegram update handler.
func NewHandler(model model.LLM, allowedUsers string) (func(context.Context, *bot.Bot, *models.Update), error) {
	// 1. Initialize the ADK Agent
	a, err := llmagent.New(llmagent.Config{
		Name:        "dross_assistant",
		Model:       model,
		Description: "Your personal, extendable assistant running on your Mac via Ollama.",
		Instruction: "You are Dross, a helpful, intelligent personal assistant. Keep your answers clear, elegant, and concise. Format your answers in Markdown when appropriate.",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// 2. Initialize in-memory session service and runner
	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:           "dross",
		Agent:             a,
		SessionService:    sessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	h := &telegramHandler{
		runner:       r,
		allowedUsers: allowedUsers,
	}

	return h.handleUpdate, nil
}

func (h *telegramHandler) handleUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	msg := update.Message
	userID := msg.From.ID
	chatID := msg.Chat.ID
	threadID := msg.MessageThreadID

	// 1. Authorization check
	if !h.isAllowed(userID) {
		log.Printf("UNAUTHORIZED access attempt from User ID: %d, Username: @%s, Name: %s %s in Chat ID: %d",
			userID, msg.From.Username, msg.From.FirstName, msg.From.LastName, chatID)

		// Send helpful reply so the user can easily copy their ID
		deniedText := fmt.Sprintf("❌ <b>Access Denied</b>\nYour Telegram User ID is: <code>%d</code>\nAdd this ID to <code>TELEGRAM_ALLOWED_USERS</code> in your env file to authorize access.", userID)
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Text:            deniedText,
			ParseMode:       models.ParseModeHTML,
		})
		if err != nil {
			log.Printf("failed to send unauthorized reply: %v", err)
		}
		return
	}

	log.Printf("Received message from User ID %d in Chat ID %d (Thread %d): %q", userID, chatID, threadID, msg.Text)

	// 2. Session ID resolution
	// Private DM uses the chat ID (which is the user's ID).
	// Group topic threads use ChatID-ThreadID to keep conversations separated.
	var sessionID string
	if msg.Chat.Type == models.ChatTypePrivate {
		sessionID = fmt.Sprintf("private-%d", chatID)
	} else {
		sessionID = fmt.Sprintf("group-%d-%d", chatID, threadID)
	}

	// 3. AI response generation with streaming draft animation
	draftID := uuid.NewString()
	userMsg := genai.NewContentFromText(msg.Text, genai.RoleUser)

	// Send an initial draft placeholder
	_, _ = b.SendMessageDraft(ctx, &bot.SendMessageDraftParams{
		ChatID:          chatID,
		MessageThreadID: threadID,
		DraftID:         draftID,
		Text:            "Thinking...",
	})

	var accumulatedText strings.Builder
	var lastUpdate time.Time

	// Run agent session
	err := func() error {
		for event, err := range h.runner.Run(ctx, fmt.Sprintf("%d", userID), sessionID, userMsg, agent.RunConfig{
			StreamingMode: agent.StreamingModeSSE,
		}) {
			if err != nil {
				return err
			}
			if event.LLMResponse.Content == nil {
				continue
			}

			chunk := ""
			for _, p := range event.LLMResponse.Content.Parts {
				chunk += p.Text
			}

			if !event.IsFinalResponse() {
				accumulatedText.WriteString(chunk)
				now := time.Now()
				// Limit updates to Telegram to once every 1.5 seconds to respect rate limits
				if now.Sub(lastUpdate) >= 1500*time.Millisecond {
					_, _ = b.SendMessageDraft(ctx, &bot.SendMessageDraftParams{
						ChatID:          chatID,
						MessageThreadID: threadID,
						DraftID:         draftID,
						Text:            accumulatedText.String(),
					})
					lastUpdate = now
				}
			} else {
				// Commit the final response
				finalText := chunk
				if finalText == "" {
					finalText = accumulatedText.String()
				}
				if finalText == "" {
					finalText = "*(Empty response)*"
				}

				_, err = b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID:          chatID,
					MessageThreadID: threadID,
					Text:            finalText,
				})
				if err != nil {
					return err
				}
			}
		}
		return nil
	}()

	if err != nil {
		log.Printf("error executing agent: %v", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: threadID,
			Text:            fmt.Sprintf("⚠️ Error: Failed to generate response: %v", err),
		})
	}
}

func (h *telegramHandler) isAllowed(userID int64) bool {
	if h.allowedUsers == "" {
		return false
	}
	idStr := fmt.Sprintf("%d", userID)
	for _, val := range strings.Split(h.allowedUsers, ",") {
		if strings.TrimSpace(val) == idStr {
			return true
		}
	}
	return false
}
