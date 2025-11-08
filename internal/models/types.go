package models

import "time"

// Agent represents an AI voice agent configuration
type Agent struct {
	ID          string    `json:"id" firestore:"id"`
	Name        string    `json:"name" firestore:"name"`
	PhoneNumber string    `json:"phone_number" firestore:"phone_number"`
	Description string    `json:"description" firestore:"description"`
	Prompt      string    `json:"prompt" firestore:"prompt"`
	VoiceModel  string    `json:"voice_model" firestore:"voice_model"`
	Language    string    `json:"language" firestore:"language"`
	Active      bool      `json:"active" firestore:"active"`
	CreatedAt   time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" firestore:"updated_at"`
	OwnerID     string    `json:"owner_id" firestore:"owner_id"`
}

// Call represents a phone call record
type Call struct {
	ID              string    `json:"id" firestore:"id" bigquery:"id"`
	AgentID         string    `json:"agent_id" firestore:"agent_id" bigquery:"agent_id"`
	CallerNumber    string    `json:"caller_number" firestore:"caller_number" bigquery:"caller_number"`
	CallSID         string    `json:"call_sid" firestore:"call_sid" bigquery:"call_sid"`
	Status          string    `json:"status" firestore:"status" bigquery:"status"`
	Duration        int       `json:"duration" firestore:"duration" bigquery:"duration"`
	StartTime       time.Time `json:"start_time" firestore:"start_time" bigquery:"start_time"`
	EndTime         time.Time `json:"end_time" firestore:"end_time" bigquery:"end_time"`
	Transcript      string    `json:"transcript" firestore:"transcript" bigquery:"transcript"`
	Summary         string    `json:"summary" firestore:"summary" bigquery:"summary"`
	Cost            float64   `json:"cost" firestore:"cost" bigquery:"cost"`
	RecordingURL    string    `json:"recording_url" firestore:"recording_url" bigquery:"recording_url"`
}

// Session represents an active call session
type Session struct {
	ID              string                 `json:"id" firestore:"id"`
	CallID          string                 `json:"call_id" firestore:"call_id"`
	AgentID         string                 `json:"agent_id" firestore:"agent_id"`
	ConversationHistory []Message          `json:"conversation_history" firestore:"conversation_history"`
	Context         map[string]interface{} `json:"context" firestore:"context"`
	CreatedAt       time.Time              `json:"created_at" firestore:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" firestore:"updated_at"`
}

// Message represents a single message in a conversation
type Message struct {
	Role      string    `json:"role" firestore:"role"`
	Content   string    `json:"content" firestore:"content"`
	Timestamp time.Time `json:"timestamp" firestore:"timestamp"`
}

// User represents an admin user
type User struct {
	ID            string    `json:"id" firestore:"id"`
	Email         string    `json:"email" firestore:"email"`
	Name          string    `json:"name" firestore:"name"`
	Picture       string    `json:"picture" firestore:"picture"`
	Role          string    `json:"role" firestore:"role"`
	CreatedAt     time.Time `json:"created_at" firestore:"created_at"`
	LastLoginAt   time.Time `json:"last_login_at" firestore:"last_login_at"`
}

// Payment represents a payment transaction
type Payment struct {
	ID            string    `json:"id" firestore:"id" bigquery:"id"`
	UserID        string    `json:"user_id" firestore:"user_id" bigquery:"user_id"`
	Amount        float64   `json:"amount" firestore:"amount" bigquery:"amount"`
	Currency      string    `json:"currency" firestore:"currency" bigquery:"currency"`
	Status        string    `json:"status" firestore:"status" bigquery:"status"`
	PaymentMethod string    `json:"payment_method" firestore:"payment_method" bigquery:"payment_method"`
	Description   string    `json:"description" firestore:"description" bigquery:"description"`
	CreatedAt     time.Time `json:"created_at" firestore:"created_at" bigquery:"created_at"`
}

// UsageHistory represents usage statistics
type UsageHistory struct {
	ID            string    `json:"id" bigquery:"id"`
	UserID        string    `json:"user_id" bigquery:"user_id"`
	AgentID       string    `json:"agent_id" bigquery:"agent_id"`
	Date          time.Time `json:"date" bigquery:"date"`
	TotalCalls    int       `json:"total_calls" bigquery:"total_calls"`
	TotalDuration int       `json:"total_duration" bigquery:"total_duration"`
	TotalCost     float64   `json:"total_cost" bigquery:"total_cost"`
}

// LiveUsage represents real-time usage data
type LiveUsage struct {
	AgentID       string    `json:"agent_id"`
	ActiveCalls   int       `json:"active_calls"`
	CallsToday    int       `json:"calls_today"`
	DurationToday int       `json:"duration_today"`
	CostToday     float64   `json:"cost_today"`
	Timestamp     time.Time `json:"timestamp"`
}

// TwilioWebhookRequest represents incoming Twilio webhook data
type TwilioWebhookRequest struct {
	CallSid       string `form:"CallSid"`
	AccountSid    string `form:"AccountSid"`
	From          string `form:"From"`
	To            string `form:"To"`
	CallStatus    string `form:"CallStatus"`
	Direction     string `form:"Direction"`
	SpeechResult  string `form:"SpeechResult"`
	Confidence    string `form:"Confidence"`
}

// TwilioResponse represents TwiML response
type TwilioResponse struct {
	XMLName xml.Name `xml:"Response"`
	Say     []Say    `xml:"Say,omitempty"`
	Gather  *Gather  `xml:"Gather,omitempty"`
	Hangup  *Hangup  `xml:"Hangup,omitempty"`
}

type Say struct {
	Voice    string `xml:"voice,attr,omitempty"`
	Language string `xml:"language,attr,omitempty"`
	Text     string `xml:",chardata"`
}

type Gather struct {
	Input      string `xml:"input,attr"`
	Action     string `xml:"action,attr"`
	Method     string `xml:"method,attr"`
	Timeout    int    `xml:"timeout,attr,omitempty"`
	NumDigits  int    `xml:"numDigits,attr,omitempty"`
	Say        *Say   `xml:"Say,omitempty"`
}

type Hangup struct{}

// Config represents application configuration
type Config struct {
	GCPProjectID     string
	GCPRegion        string
	GCPProjectNumber string
	Port             string
	Environment      string
	
	TwilioAccountSID   string
	TwilioAuthToken    string
	TwilioPhoneNumber  string
	TwilioAPIKeySID    string
	TwilioAPIKeySecret string
	
	BigQueryDataset      string
	BigQueryCallsTable   string
	BigQueryUsageTable   string
	BigQueryPaymentsTable string
	
	FirestoreCollection         string
	FirestoreSessionsCollection string
	
	VertexAILocation string
	VertexAIModel    string
	VertexAIEndpoint string
	
	OAuthClientID     string
	OAuthClientSecret string
	OAuthRedirectURL  string
	OAuthStateSecret  string
	
	JWTSecret     string
	JWTExpiration string
	
	AllowedOrigins      []string
	LogLevel            string
	LogFormat           string
	RateLimitRequests   int
	RateLimitWindow     string
	
	ExternalAgentAPIURL string
	ExternalAgentAPIKey string
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// PaginationParams represents pagination parameters
type PaginationParams struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"page_size" json:"page_size"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Success    bool        `json:"success"`
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalCount int64       `json:"total_count"`
	TotalPages int         `json:"total_pages"`
}
