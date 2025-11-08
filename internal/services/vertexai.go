package services

import (
	"context"
	"fmt"

	"cloud.google.com/go/vertexai/genai"
	"github.com/ai-atl-dev/HeyAI-backend/internal/models"
)

// VertexAIService handles interactions with Vertex AI
type VertexAIService struct {
	client *genai.Client
	model  string
	config *models.Config
}

// NewVertexAIService creates a new Vertex AI service
func NewVertexAIService(ctx context.Context, config *models.Config) (*VertexAIService, error) {
	client, err := genai.NewClient(ctx, config.GCPProjectID, config.VertexAILocation)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	return &VertexAIService{
		client: client,
		model:  config.VertexAIModel,
		config: config,
	}, nil
}

// Close closes the Vertex AI client
func (s *VertexAIService) Close() error {
	return s.client.Close()
}

// ProcessVoiceInput processes voice input and generates a response
func (s *VertexAIService) ProcessVoiceInput(ctx context.Context, agent *models.Agent, userInput string, session *models.Session) (string, error) {
	model := s.client.GenerativeModel(s.model)

	// Configure model parameters
	model.SetTemperature(0.7)
	model.SetTopP(0.95)
	model.SetTopK(40)
	model.SetMaxOutputTokens(1024)

	// Set system instruction if agent has custom prompt
	if agent.Prompt != "" {
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(agent.Prompt)},
		}
	}

	// Start chat session with history
	chat := model.StartChat()
	
	// Add conversation history
	for _, msg := range session.ConversationHistory {
		role := "user"
		if msg.Role == "assistant" || msg.Role == "model" {
			role = "model"
		}
		chat.History = append(chat.History, &genai.Content{
			Parts: []genai.Part{genai.Text(msg.Content)},
			Role:  role,
		})
	}

	// Send user message
	resp, err := chat.SendMessage(ctx, genai.Text(userInput))
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no response generated")
	}

	// Extract text from response
	var responseText string
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			responseText += string(text)
		}
	}

	return responseText, nil
}

// GenerateSummary generates a summary of a conversation
func (s *VertexAIService) GenerateSummary(ctx context.Context, conversationHistory []models.Message) (string, error) {
	model := s.client.GenerativeModel(s.model)
	model.SetTemperature(0.3)
	model.SetMaxOutputTokens(256)

	// Build conversation text
	var conversationText string
	for _, msg := range conversationHistory {
		conversationText += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}

	prompt := fmt.Sprintf("Please provide a concise summary of the following conversation:\n\n%s\n\nSummary:", conversationText)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no summary generated")
	}

	var summary string
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			summary += string(text)
		}
	}

	return summary, nil
}
