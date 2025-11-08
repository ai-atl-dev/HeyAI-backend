package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ai-atl-dev/HeyAI-backend/internal/middleware"
	"github.com/ai-atl-dev/HeyAI-backend/internal/models"
	"github.com/ai-atl-dev/HeyAI-backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	firestoreService *services.FirestoreService
	authMiddleware   *middleware.AuthMiddleware
	oauthConfig      *oauth2.Config
	stateSecret      string
	jwtExpiration    time.Duration
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(
	firestoreService *services.FirestoreService,
	authMiddleware *middleware.AuthMiddleware,
	config *models.Config,
) *AuthHandler {
	oauthConfig := &oauth2.Config{
		ClientID:     config.OAuthClientID,
		ClientSecret: config.OAuthClientSecret,
		RedirectURL:  config.OAuthRedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	jwtExpiration, _ := time.ParseDuration(config.JWTExpiration)
	if jwtExpiration == 0 {
		jwtExpiration = 24 * time.Hour
	}

	return &AuthHandler{
		firestoreService: firestoreService,
		authMiddleware:   authMiddleware,
		oauthConfig:      oauthConfig,
		stateSecret:      config.OAuthStateSecret,
		jwtExpiration:    jwtExpiration,
	}
}

// GoogleUserInfo represents user info from Google
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

// Login initiates OAuth login flow
func (h *AuthHandler) Login(c *gin.Context) {
	state := h.generateState()
	
	// Store state in session/cookie for validation (simplified here)
	c.SetCookie("oauth_state", state, 600, "/", "", false, true)

	url := h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	
	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data: gin.H{
			"auth_url": url,
		},
	})
}

// Callback handles OAuth callback
func (h *AuthHandler) Callback(c *gin.Context) {
	// Validate state
	state := c.Query("state")
	storedState, err := c.Cookie("oauth_state")
	if err != nil || state != storedState {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Invalid state parameter",
		})
		return
	}

	// Clear state cookie
	c.SetCookie("oauth_state", "", -1, "/", "", false, true)

	// Exchange code for token
	code := c.Query("code")
	token, err := h.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to exchange token",
		})
		return
	}

	// Get user info from Google
	userInfo, err := h.getUserInfo(token.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get user info",
		})
		return
	}

	// Create or update user in Firestore
	user, err := h.createOrUpdateUser(c.Request.Context(), userInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to create/update user",
		})
		return
	}

	// Generate JWT token
	jwtToken, err := h.authMiddleware.GenerateToken(user, h.jwtExpiration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to generate token",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data: gin.H{
			"token": jwtToken,
			"user":  user,
		},
	})
}

// Me returns current user info
func (h *AuthHandler) Me(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Success: false,
			Error:   "User not authenticated",
		})
		return
	}

	user, err := h.firestoreService.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    user,
	})
}

// Logout logs out the user (client-side token removal)
func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Logged out successfully",
	})
}

// RefreshToken refreshes the JWT token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Success: false,
			Error:   "User not authenticated",
		})
		return
	}

	user, err := h.firestoreService.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	// Generate new JWT token
	jwtToken, err := h.authMiddleware.GenerateToken(user, h.jwtExpiration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to generate token",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data: gin.H{
			"token": jwtToken,
		},
	})
}

// Helper functions

func (h *AuthHandler) generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (h *AuthHandler) getUserInfo(accessToken string) (*GoogleUserInfo, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user info: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &userInfo, nil
}

func (h *AuthHandler) createOrUpdateUser(ctx context.Context, googleUser *GoogleUserInfo) (*models.User, error) {
	// Try to get existing user
	existingUser, err := h.firestoreService.GetUserByEmail(ctx, googleUser.Email)
	if err == nil {
		// User exists, update last login
		existingUser.LastLoginAt = time.Now()
		if err := h.firestoreService.UpdateUser(ctx, existingUser); err != nil {
			return nil, err
		}
		return existingUser, nil
	}

	// Create new user
	user := &models.User{
		ID:          uuid.New().String(),
		Email:       googleUser.Email,
		Name:        googleUser.Name,
		Picture:     googleUser.Picture,
		Role:        "user", // Default role
		CreatedAt:   time.Now(),
		LastLoginAt: time.Now(),
	}

	if err := h.firestoreService.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}
