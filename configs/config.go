package configs

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/yourusername/dashboard-backend/internal/models"
)

// LoadConfig loads configuration from environment variables
func LoadConfig() (*models.Config, error) {
	config := &models.Config{
		GCPProjectID:     getEnv("GCP_PROJECT_ID", ""),
		GCPRegion:        getEnv("GCP_REGION", "us-central1"),
		GCPProjectNumber: getEnv("GCP_PROJECT_NUMBER", ""),
		Port:             getEnv("PORT", "8080"),
		Environment:      getEnv("ENVIRONMENT", "development"),

		TwilioAccountSID:   getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:    getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioPhoneNumber:  getEnv("TWILIO_PHONE_NUMBER", ""),
		TwilioAPIKeySID:    getEnv("TWILIO_API_KEY_SID", ""),
		TwilioAPIKeySecret: getEnv("TWILIO_API_KEY_SECRET", ""),

		BigQueryDataset:       getEnv("BIGQUERY_DATASET", "agent_data"),
		BigQueryCallsTable:    getEnv("BIGQUERY_CALLS_TABLE", "calls"),
		BigQueryUsageTable:    getEnv("BIGQUERY_USAGE_TABLE", "usage_history"),
		BigQueryPaymentsTable: getEnv("BIGQUERY_PAYMENTS_TABLE", "payments"),

		FirestoreCollection:         getEnv("FIRESTORE_COLLECTION", "agents"),
		FirestoreSessionsCollection: getEnv("FIRESTORE_SESSIONS_COLLECTION", "sessions"),

		VertexAILocation: getEnv("VERTEX_AI_LOCATION", "us-central1"),
		VertexAIModel:    getEnv("VERTEX_AI_MODEL", "gemini-2.5-flash"),
		VertexAIEndpoint: getEnv("VERTEX_AI_ENDPOINT", ""),

		OAuthClientID:     getEnv("OAUTH_CLIENT_ID", ""),
		OAuthClientSecret: getEnv("OAUTH_CLIENT_SECRET", ""),
		OAuthRedirectURL:  getEnv("OAUTH_REDIRECT_URL", "http://localhost:8080/auth/callback"),
		OAuthStateSecret:  getEnv("OAUTH_STATE_SECRET", ""),

		JWTSecret:     getEnv("JWT_SECRET", ""),
		JWTExpiration: getEnv("JWT_EXPIRATION", "24h"),

		AllowedOrigins:    parseStringSlice(getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		LogFormat:         getEnv("LOG_FORMAT", "json"),
		RateLimitRequests: getEnvAsInt("RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:   getEnv("RATE_LIMIT_WINDOW", "1m"),

		ExternalAgentAPIURL: getEnv("EXTERNAL_AGENT_API_URL", ""),
		ExternalAgentAPIKey: getEnv("EXTERNAL_AGENT_API_KEY", ""),
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// validateConfig validates required configuration fields
func validateConfig(config *models.Config) error {
	required := map[string]string{
		"GCP_PROJECT_ID":      config.GCPProjectID,
		"TWILIO_ACCOUNT_SID":  config.TwilioAccountSID,
		"TWILIO_AUTH_TOKEN":   config.TwilioAuthToken,
		"TWILIO_PHONE_NUMBER": config.TwilioPhoneNumber,
	}

	var missing []string
	for key, value := range required {
		if value == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable as an integer with a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

// parseStringSlice parses a comma-separated string into a slice
func parseStringSlice(s string) []string {
	if s == "" {
		return []string{}
	}

	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
