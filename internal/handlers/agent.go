package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ai-atl-dev/HeyAI-backend/internal/models"
	"github.com/ai-atl-dev/HeyAI-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AgentHandler handles Twilio webhook endpoints
type AgentHandler struct {
	firestoreService *services.FirestoreService
	twilioService    *services.TwilioService
	vertexAIService  *services.VertexAIService
	bigQueryService  *services.BigQueryService
	baseURL          string
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(
	firestoreService *services.FirestoreService,
	twilioService *services.TwilioService,
	vertexAIService *services.VertexAIService,
	bigQueryService *services.BigQueryService,
	baseURL string,
) *AgentHandler {
	return &AgentHandler{
		firestoreService: firestoreService,
		twilioService:    twilioService,
		vertexAIService:  vertexAIService,
		bigQueryService:  bigQueryService,
		baseURL:          baseURL,
	}
}

// VoiceWebhook handles incoming Twilio voice calls
func (h *AgentHandler) VoiceWebhook(c *gin.Context) {
	var req models.TwilioWebhookRequest
	if err := c.ShouldBind(&req); err != nil {
		c.String(http.StatusBadRequest, "Invalid request")
		return
	}

	// Get agent by phone number (the number that was called)
	agent, err := h.firestoreService.GetAgentByPhoneNumber(c.Request.Context(), req.To)
	if err != nil {
		// No agent found, use default response
		twiml, _ := h.twilioService.GenerateHangupResponse("Sorry, this service is not available.")
		c.Header("Content-Type", "text/xml")
		c.String(http.StatusOK, twiml)
		return
	}

	// Check if agent is active
	if !agent.Active {
		twiml, _ := h.twilioService.GenerateHangupResponse("This agent is currently unavailable.")
		c.Header("Content-Type", "text/xml")
		c.String(http.StatusOK, twiml)
		return
	}

	// Create or get session
	session, err := h.getOrCreateSession(c.Request.Context(), req.CallSid, agent.ID)
	if err != nil {
		twiml, _ := h.twilioService.GenerateHangupResponse("An error occurred. Please try again.")
		c.Header("Content-Type", "text/xml")
		c.String(http.StatusOK, twiml)
		return
	}

	// Handle different call statuses
	switch req.CallStatus {
	case "ringing", "in-progress":
		h.handleInProgressCall(c, &req, agent, session)
	case "completed":
		h.handleCompletedCall(c, &req, agent, session)
	default:
		h.handleInProgressCall(c, &req, agent, session)
	}
}

// handleInProgressCall handles active calls
func (h *AgentHandler) handleInProgressCall(c *gin.Context, req *models.TwilioWebhookRequest, agent *models.Agent, session *models.Session) {
	ctx := c.Request.Context()

	// If this is the first interaction (no speech result yet)
	if req.SpeechResult == "" {
		// Generate greeting
		greeting := fmt.Sprintf("Hello! You've reached %s. How can I help you today?", agent.Name)
		
		// Generate TwiML to gather speech
		gatherAction := fmt.Sprintf("%s/webhook/voice", h.baseURL)
		twiml, err := h.twilioService.GenerateTwiMLResponse(greeting, true, gatherAction)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error generating response")
			return
		}

		c.Header("Content-Type", "text/xml")
		c.String(http.StatusOK, twiml)
		return
	}

	// User has spoken, add to conversation history
	userMessage := models.Message{
		Role:      "user",
		Content:   req.SpeechResult,
		Timestamp: time.Now(),
	}
	session.ConversationHistory = append(session.ConversationHistory, userMessage)

	// Generate AI response using Vertex AI
	aiResponse, err := h.vertexAIService.ProcessVoiceInput(ctx, agent, req.SpeechResult, session)
	if err != nil {
		twiml, _ := h.twilioService.GenerateHangupResponse("I'm having trouble processing your request. Please try again later.")
		c.Header("Content-Type", "text/xml")
		c.String(http.StatusOK, twiml)
		return
	}

	// Add AI response to conversation history
	assistantMessage := models.Message{
		Role:      "assistant",
		Content:   aiResponse,
		Timestamp: time.Now(),
	}
	session.ConversationHistory = append(session.ConversationHistory, assistantMessage)

	// Update session in Firestore
	if err := h.firestoreService.UpdateSession(ctx, session); err != nil {
		// Log error but continue
		fmt.Printf("Failed to update session: %v\n", err)
	}

	// Check if conversation should end (simple heuristic)
	shouldEnd := h.shouldEndConversation(aiResponse)

	var twiml string
	if shouldEnd {
		twiml, _ = h.twilioService.GenerateHangupResponse(aiResponse)
	} else {
		gatherAction := fmt.Sprintf("%s/webhook/voice", h.baseURL)
		twiml, _ = h.twilioService.GenerateTwiMLResponse(aiResponse, true, gatherAction)
	}

	c.Header("Content-Type", "text/xml")
	c.String(http.StatusOK, twiml)
}

// handleCompletedCall handles call completion
func (h *AgentHandler) handleCompletedCall(c *gin.Context, req *models.TwilioWebhookRequest, agent *models.Agent, session *models.Session) {
	ctx := c.Request.Context()

	// Get call details from Twilio
	callDetails, err := h.twilioService.GetCallDetails(req.CallSid)
	if err != nil {
		fmt.Printf("Failed to get call details: %v\n", err)
	}

	// Generate summary
	summary := ""
	if len(session.ConversationHistory) > 0 {
		summary, _ = h.vertexAIService.GenerateSummary(ctx, session.ConversationHistory)
	}

	// Build transcript
	transcript := ""
	for _, msg := range session.ConversationHistory {
		transcript += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}

	// Create call record
	call := &models.Call{
		ID:           uuid.New().String(),
		AgentID:      agent.ID,
		CallerNumber: req.From,
		CallSID:      req.CallSid,
		Status:       "completed",
		StartTime:    time.Now().Add(-time.Duration(*callDetails.Duration) * time.Second),
		EndTime:      time.Now(),
		Duration:     int(*callDetails.Duration),
		Transcript:   transcript,
		Summary:      summary,
		Cost:         0.0, // Calculate based on duration
	}

	// Save call to BigQuery
	if err := h.bigQueryService.InsertCall(ctx, call); err != nil {
		fmt.Printf("Failed to insert call to BigQuery: %v\n", err)
	}

	// Clean up session
	if err := h.firestoreService.DeleteSession(ctx, session.ID); err != nil {
		fmt.Printf("Failed to delete session: %v\n", err)
	}

	c.String(http.StatusOK, "")
}

// getOrCreateSession gets or creates a session for a call
func (h *AgentHandler) getOrCreateSession(ctx context.Context, callSID, agentID string) (*models.Session, error) {
	// Try to get existing session
	session, err := h.firestoreService.GetSessionByCallID(ctx, callSID)
	if err == nil {
		return session, nil
	}

	// Create new session
	session = &models.Session{
		ID:                  uuid.New().String(),
		CallID:              callSID,
		AgentID:             agentID,
		ConversationHistory: []models.Message{},
		Context:             make(map[string]interface{}),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	if err := h.firestoreService.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

// shouldEndConversation determines if the conversation should end
func (h *AgentHandler) shouldEndConversation(response string) bool {
	// Simple heuristic - check for goodbye phrases
	goodbyePhrases := []string{
		"goodbye",
		"bye",
		"have a great day",
		"take care",
		"talk to you later",
	}

	responseLower := strings.ToLower(response)
	for _, phrase := range goodbyePhrases {
		if strings.Contains(responseLower, phrase) {
			return true
		}
	}

	return false
}
