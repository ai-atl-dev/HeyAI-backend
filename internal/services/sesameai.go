package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ai-atl-dev/HeyAI-backend/internal/models"
)

// SesameAIService handles voice synthesis with Sesame AI
type SesameAIService struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewSesameAIService creates a new Sesame AI service
func NewSesameAIService(config *models.Config) *SesameAIService {
	baseURL := config.SesameAIBaseURL
	if baseURL == "" {
		baseURL = "https://api.sesame.ai/v1" // Default endpoint
	}

	return &SesameAIService{
		apiKey:  config.SesameAIAPIKey,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

// TTSRequest represents a text-to-speech request
type SesameAITTSRequest struct {
	Text     string                 `json:"text"`
	VoiceID  string                 `json:"voice_id,omitempty"`
	Language string                 `json:"language,omitempty"`
	Speed    float64                `json:"speed,omitempty"`
	Pitch    float64                `json:"pitch,omitempty"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// STTRequest represents a speech-to-text request
type SesameAISTTRequest struct {
	AudioURL string `json:"audio_url,omitempty"`
	Language string `json:"language,omitempty"`
}

// TTSResponse represents the TTS response
type SesameAITTSResponse struct {
	AudioURL  string `json:"audio_url"`
	AudioData []byte `json:"audio_data,omitempty"`
	Duration  float64 `json:"duration"`
	Status    string `json:"status"`
}

// STTResponse represents the STT response
type SesameAISTTResponse struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Language   string  `json:"language"`
	Duration   float64 `json:"duration"`
}

// ConversationRequest represents a full conversation request
type SesameAIConversationRequest struct {
	Text          string                 `json:"text"`
	AudioInput    string                 `json:"audio_input,omitempty"`
	VoiceID       string                 `json:"voice_id,omitempty"`
	SystemPrompt  string                 `json:"system_prompt,omitempty"`
	Context       []map[string]string    `json:"context,omitempty"`
	Settings      map[string]interface{} `json:"settings,omitempty"`
}

// ConversationResponse represents the conversation response
type SesameAIConversationResponse struct {
	Text      string  `json:"text"`
	AudioURL  string  `json:"audio_url"`
	AudioData []byte  `json:"audio_data,omitempty"`
	Duration  float64 `json:"duration"`
	Status    string  `json:"status"`
}

// TextToSpeech converts text to speech
func (s *SesameAIService) TextToSpeech(text, voiceID, language string) (*SesameAITTSResponse, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("Sesame AI API key not configured")
	}

	url := fmt.Sprintf("%s/tts", s.baseURL)

	requestBody := SesameAITTSRequest{
		Text:     text,
		VoiceID:  voiceID,
		Language: language,
		Speed:    1.0,
		Pitch:    1.0,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var ttsResp SesameAITTSResponse
	if err := json.NewDecoder(resp.Body).Decode(&ttsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &ttsResp, nil
}

// SpeechToText converts speech to text
func (s *SesameAIService) SpeechToText(audioURL, language string) (*SesameAISTTResponse, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("Sesame AI API key not configured")
	}

	url := fmt.Sprintf("%s/stt", s.baseURL)

	requestBody := SesameAISTTRequest{
		AudioURL: audioURL,
		Language: language,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var sttResp SesameAISTTResponse
	if err := json.NewDecoder(resp.Body).Decode(&sttResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &sttResp, nil
}

// ProcessConversation handles full conversation flow (STT + LLM + TTS)
func (s *SesameAIService) ProcessConversation(req *SesameAIConversationRequest) (*SesameAIConversationResponse, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("Sesame AI API key not configured")
	}

	url := fmt.Sprintf("%s/conversation", s.baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var convResp SesameAIConversationResponse
	if err := json.NewDecoder(resp.Body).Decode(&convResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &convResp, nil
}

// StreamConversation streams conversation response (for real-time use)
func (s *SesameAIService) StreamConversation(req *SesameAIConversationRequest) (io.ReadCloser, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("Sesame AI API key not configured")
	}

	url := fmt.Sprintf("%s/conversation/stream", s.baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return resp.Body, nil
}

// GetDefaultVoiceID returns a default voice ID
func (s *SesameAIService) GetDefaultVoiceID() string {
	return "default-voice"
}

// HealthCheck checks if Sesame AI service is available
func (s *SesameAIService) HealthCheck() error {
	if s.apiKey == "" {
		return fmt.Errorf("Sesame AI API key not configured")
	}

	url := fmt.Sprintf("%s/health", s.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}
