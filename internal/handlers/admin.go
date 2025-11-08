package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ai-atl-dev/HeyAI-backend/internal/middleware"
	"github.com/ai-atl-dev/HeyAI-backend/internal/models"
	"github.com/ai-atl-dev/HeyAI-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AdminHandler handles admin console endpoints
type AdminHandler struct {
	firestoreService *services.FirestoreService
	bigQueryService  *services.BigQueryService
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(
	firestoreService *services.FirestoreService,
	bigQueryService *services.BigQueryService,
) *AdminHandler {
	return &AdminHandler{
		firestoreService: firestoreService,
		bigQueryService:  bigQueryService,
	}
}

// Agent CRUD Operations

// CreateAgent creates a new agent
func (h *AdminHandler) CreateAgent(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Success: false,
			Error:   "Unauthorized",
		})
		return
	}

	var agent models.Agent
	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	agent.ID = uuid.New().String()
	agent.OwnerID = userID
	agent.Active = true
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()

	if err := h.firestoreService.CreateAgent(c.Request.Context(), &agent); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to create agent",
		})
		return
	}

	c.JSON(http.StatusCreated, models.APIResponse{
		Success: true,
		Data:    agent,
		Message: "Agent created successfully",
	})
}

// GetAgent retrieves an agent by ID
func (h *AdminHandler) GetAgent(c *gin.Context) {
	agentID := c.Param("id")

	agent, err := h.firestoreService.GetAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Success: false,
			Error:   "Agent not found",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    agent,
	})
}

// ListAgents retrieves all agents for the current user
func (h *AdminHandler) ListAgents(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Success: false,
			Error:   "Unauthorized",
		})
		return
	}

	agents, err := h.firestoreService.ListAgents(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to retrieve agents",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    agents,
	})
}

// UpdateAgent updates an existing agent
func (h *AdminHandler) UpdateAgent(c *gin.Context) {
	agentID := c.Param("id")

	var agent models.Agent
	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	agent.ID = agentID
	agent.UpdatedAt = time.Now()

	if err := h.firestoreService.UpdateAgent(c.Request.Context(), &agent); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to update agent",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    agent,
		Message: "Agent updated successfully",
	})
}

// DeleteAgent deletes an agent
func (h *AdminHandler) DeleteAgent(c *gin.Context) {
	agentID := c.Param("id")

	if err := h.firestoreService.DeleteAgent(c.Request.Context(), agentID); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to delete agent",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Agent deleted successfully",
	})
}

// Usage History Operations

// GetUsageHistory retrieves usage history
func (h *AdminHandler) GetUsageHistory(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Success: false,
			Error:   "Unauthorized",
		})
		return
	}

	limit := 30
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	usage, err := h.bigQueryService.GetUsageHistory(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to retrieve usage history",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    usage,
	})
}

// GetAgentStats retrieves statistics for an agent
func (h *AdminHandler) GetAgentStats(c *gin.Context) {
	agentID := c.Param("id")

	// Default to last 30 days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	if startStr := c.Query("start_date"); startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			startDate = t
		}
	}

	if endStr := c.Query("end_date"); endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			endDate = t
		}
	}

	stats, err := h.bigQueryService.GetCallStats(c.Request.Context(), agentID, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to retrieve stats",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    stats,
	})
}

// GetCallHistory retrieves call history for an agent
func (h *AdminHandler) GetCallHistory(c *gin.Context) {
	agentID := c.Param("id")

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	calls, err := h.bigQueryService.GetCallsByAgent(c.Request.Context(), agentID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to retrieve call history",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    calls,
	})
}

// GetCallTrends retrieves call trends
func (h *AdminHandler) GetCallTrends(c *gin.Context) {
	agentID := c.Param("id")

	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil {
			days = d
		}
	}

	trends, err := h.bigQueryService.GetCallTrends(c.Request.Context(), agentID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to retrieve trends",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    trends,
	})
}

// GetTopCallers retrieves top callers for an agent
func (h *AdminHandler) GetTopCallers(c *gin.Context) {
	agentID := c.Param("id")

	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	callers, err := h.bigQueryService.GetTopCallers(c.Request.Context(), agentID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to retrieve top callers",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    callers,
	})
}

// Payment Operations

// CreatePayment creates a payment record
func (h *AdminHandler) CreatePayment(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Success: false,
			Error:   "Unauthorized",
		})
		return
	}

	var payment models.Payment
	if err := c.ShouldBindJSON(&payment); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	payment.ID = uuid.New().String()
	payment.UserID = userID
	payment.CreatedAt = time.Now()

	if err := h.bigQueryService.InsertPayment(c.Request.Context(), &payment); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to create payment",
		})
		return
	}

	c.JSON(http.StatusCreated, models.APIResponse{
		Success: true,
		Data:    payment,
		Message: "Payment recorded successfully",
	})
}

// GetPaymentHistory retrieves payment history
func (h *AdminHandler) GetPaymentHistory(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Success: false,
			Error:   "Unauthorized",
		})
		return
	}

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	payments, err := h.bigQueryService.GetPaymentsByUser(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to retrieve payment history",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    payments,
	})
}

// Live Usage Streaming

// StreamLiveUsage streams live usage data (Server-Sent Events)
func (h *AdminHandler) StreamLiveUsage(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Success: false,
			Error:   "Unauthorized",
		})
		return
	}

	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Get user's agents
	agents, err := h.firestoreService.ListAgents(c.Request.Context(), userID)
	if err != nil {
		c.SSEvent("error", "Failed to retrieve agents")
		return
	}

	// Stream usage data every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			var liveUsageData []models.LiveUsage

			for _, agent := range agents {
				// Get today's usage
				usage, err := h.bigQueryService.GetDailyUsage(c.Request.Context(), agent.ID, time.Now())
				if err != nil {
					continue
				}

				liveUsage := models.LiveUsage{
					AgentID:       agent.ID,
					ActiveCalls:   0, // Would need real-time tracking
					CallsToday:    usage.TotalCalls,
					DurationToday: usage.TotalDuration,
					CostToday:     usage.TotalCost,
					Timestamp:     time.Now(),
				}

				liveUsageData = append(liveUsageData, liveUsage)
			}

			c.SSEvent("usage", liveUsageData)
			c.Writer.Flush()
		}
	}
}

// Dashboard summary
func (h *AdminHandler) GetDashboardSummary(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Success: false,
			Error:   "Unauthorized",
		})
		return
	}

	// Get user's agents
	agents, err := h.firestoreService.ListAgents(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to retrieve agents",
		})
		return
	}

	// Calculate totals
	totalAgents := len(agents)
	activeAgents := 0
	for _, agent := range agents {
		if agent.Active {
			activeAgents++
		}
	}

	// Get usage for last 30 days
	usage, _ := h.bigQueryService.GetUsageHistory(c.Request.Context(), userID, 30)
	
	totalCalls := 0
	totalCost := 0.0
	for _, u := range usage {
		totalCalls += u.TotalCalls
		totalCost += u.TotalCost
	}

	summary := gin.H{
		"total_agents":  totalAgents,
		"active_agents": activeAgents,
		"total_calls":   totalCalls,
		"total_cost":    totalCost,
		"agents":        agents,
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    summary,
	})
}
