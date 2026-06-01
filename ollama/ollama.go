// Package ollama implements the [model.LLM] interface for Ollama local models.
package ollama

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/ollama/ollama/api"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type ollamaModel struct {
	client *api.Client
	name   string
}

// NewModel returns a model.LLM backed by the Ollama API, initialized from the environment.
func NewModel(modelName string) (model.LLM, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama client: %w", err)
	}
	return &ollamaModel{
		client: client,
		name:   modelName,
	}, nil
}

func (m *ollamaModel) Name() string {
	return m.name
}

// GenerateContent calls the local Ollama chat API.
func (m *ollamaModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		var messages []api.Message

		// 1. Map System Instruction if present
		if req.Config != nil && req.Config.SystemInstruction != nil {
			var sb strings.Builder
			for _, part := range req.Config.SystemInstruction.Parts {
				sb.WriteString(part.Text)
			}
			if sb.Len() > 0 {
				messages = append(messages, api.Message{
					Role:    "system",
					Content: sb.String(),
				})
			}
		}

		// 2. Map conversational history
		for _, c := range req.Contents {
			var sb strings.Builder
			for _, part := range c.Parts {
				sb.WriteString(part.Text)
			}
			role := c.Role
			if role == "model" {
				role = "assistant"
			} else if role == "" {
				role = "user"
			}
			messages = append(messages, api.Message{
				Role:    role,
				Content: sb.String(),
			})
		}

		// Allow overriding the model dynamically via the request if needed
		modelName := m.name
		if req.Model != "" {
			modelName = req.Model
		}

		chatReq := &api.ChatRequest{
			Model:    modelName,
			Messages: messages,
			Stream:   &stream,
		}

		// 3. Perform Streaming Call
		if stream {
			err := m.client.Chat(ctx, chatReq, func(resp api.ChatResponse) error {
				llmResp := &model.LLMResponse{
					Content: &genai.Content{
						Role: "model",
						Parts: []*genai.Part{
							{Text: resp.Message.Content},
						},
					},
					Partial:      !resp.Done,
					TurnComplete: resp.Done,
				}
				if !yield(llmResp, nil) {
					return fmt.Errorf("consumer stopped yielding")
				}
				return nil
			})
			if err != nil {
				yield(nil, err)
			}
			return
		}

		// 4. Perform Non-Streaming Call
		var finalContent strings.Builder
		err := m.client.Chat(ctx, chatReq, func(resp api.ChatResponse) error {
			finalContent.WriteString(resp.Message.Content)
			return nil
		})
		if err != nil {
			yield(nil, err)
			return
		}

		llmResp := &model.LLMResponse{
			Content: &genai.Content{
				Role: "model",
				Parts: []*genai.Part{
					{Text: finalContent.String()},
				},
			},
			Partial:      false,
			TurnComplete: true,
		}
		yield(llmResp, nil)
	}
}

var _ model.LLM = &ollamaModel{}
