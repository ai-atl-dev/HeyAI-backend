package services

import (
	"context"
	"fmt"

	"cloud.google.com/go/vertexai/genai"
	"github.com/ai-atl-dev/HeyAI-backend/internal/models"
)

// VertexAIService handles interactions with Vertex AI
type VertexAIService struct {
	client   *genai.Client
	model    string
	config   *models.Config
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

// GenerateResponse generates a response from the AI model
func (s *VertexAIService) GenerateResponse(ctx context.Context, prompt string, conversationHistory []models.Message) (string, error) {
	model := s.client.GenerativeModel(s.model)

	// Configure model parameters
	model.SetTemperature(0.7)
	model.SetTopP(0.95)
	model.SetTopK(40)
	model.SetMaxOutputTokens(1024)

	// Build conversation history
	var contents []*genai.Content
	for _, msg := range conversationHistory {
		role := "user"
		if msg.Role == "assistant" || msg.Role == "model" {
			role = "model"
		}
		contents = append(contents, &genai.Content{
			Parts: []genai.Part{genai.Text(msg.Content)},
			Role:  role,
		})
	}

	// Add current prompt
	contents = append(contents, &genai.Content{
		Parts: []genai.Part{genai.Text(prompt)},
		Role:  "user",
	})

	// Generate response
	resp, err := model.GenerateContent(ctx, contents...)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no response candidates generated")
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

// GenerateResponseWithSystemPrompt generates a response with a system prompt
func (s *VertexAIService) GenerateResponseWithSystemPrompt(ctx context.Context, systemPrompt, userPrompt string, conversationHistory []models.Message) (string, error) {
	model := s.client.GenerativeModel(s.model)

	// Configure model parameters
	model.SetTemperature(0.7)
	model.SetTopP(0.95)
	model.SetTopK(40)
	model.SetMaxOutputTokens(1024)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}

	// Build conversation history
	var contents []*genai.Content
	for _, msg := range conversationHistory {
		role := "user"
		if msg.Role == "assistant" || msg.Role == "model" {
			role = "model"
		}
		contents = append(contents, &genai.Content{
			Parts: []genai.Part{genai.Text(msg.Content)},
			Role:  role,
		})
	}

	// Add current prompt
	contents = append(contents, &genai.Content{
		Parts: []genai.Part{genai.Text(userPrompt)},
		Role:  "user",
	})

	// Generate response
	resp, err := model.GenerateContent(ctx, contents...)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no response candidates generated")
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

// ProcessVoiceInput processes voice input and generates a response
func (s *VertexAIService) ProcessVoiceInput(ctx context.Context, agent *models.Agent, userInput string, session *models.Session) (string, error) {
	// Use agent's custom prompt as system instruction
	systemPrompt := agent.Prompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful AI voice assistant. Respond naturally and conversationally."
	}

	// Generate response with conversation history
	response, err := s.GenerateResponseWithSystemPrompt(ctx, systemPrompt, userInput, session.ConversationHistory)
	if err != nil {
		return "", fmt.Errorf("failed to process voice input: %w", err)
	}

	return response, nil
}

// StreamResponse streams a response from the AI model (for future use)
func (s *VertexAIService) StreamResponse(ctx context.Context, prompt string) (<-chan string, <-chan error) {
	responseChan := make(chan string)
	errorChan := make(chan error, 1)

	go func() {
		defer close(responseChan)
		defer close(errorChan)

		model := s.client.GenerativeModel(s.model)
		model.SetTemperature(0.7)

		iter := model.GenerateContentStream(ctx, genai.Text(prompt))
		for {
			resp, err := iter.Next()
			if err != nil {
				errorChan <- err
				return
			}

			for _, part := range resp.Candidates[0].Content.Parts {
				if text, ok := part.(genai.Text); ok {
					responseChan <- string(text)
				}
			}
		}
	}()

	return responseChan, errorChan
}

// GenerateSummary generates a summary of a conversation
func (s *VertexAIService) GenerateSummary(ctx context.Context, conversationHistory []models.Message) (string, error) {
	// Build conversation text
	var conversationText string
	for _, msg := range conversationHistory {
		conversationText += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}

	prompt := fmt.Sprintf("Please provide a concise summary of the following conversation:\n\n%s\n\nSummary:", conversationText)

	model := s.client.GenerativeModel(s.model)
	model.SetTemperature(0.3)
	model.SetMaxOutputTokens(256)

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
