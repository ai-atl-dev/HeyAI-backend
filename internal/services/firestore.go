package services

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/ai-atl-dev/HeyAI-backend/internal/models"
	"google.golang.org/api/iterator"
)

// FirestoreService handles Firestore database operations
type FirestoreService struct {
	client                *firestore.Client
	agentsCollection      string
	sessionsCollection    string
}

// NewFirestoreService creates a new Firestore service
func NewFirestoreService(ctx context.Context, config *models.Config) (*FirestoreService, error) {
	client, err := firestore.NewClient(ctx, config.GCPProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Firestore client: %w", err)
	}

	return &FirestoreService{
		client:             client,
		agentsCollection:   config.FirestoreCollection,
		sessionsCollection: config.FirestoreSessionsCollection,
	}, nil
}

// Close closes the Firestore client
func (s *FirestoreService) Close() error {
	return s.client.Close()
}

// Agent Operations

// CreateAgent creates a new agent
func (s *FirestoreService) CreateAgent(ctx context.Context, agent *models.Agent) error {
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()

	_, err := s.client.Collection(s.agentsCollection).Doc(agent.ID).Set(ctx, agent)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return nil
}

// GetAgent retrieves an agent by ID
func (s *FirestoreService) GetAgent(ctx context.Context, agentID string) (*models.Agent, error) {
	doc, err := s.client.Collection(s.agentsCollection).Doc(agentID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	var agent models.Agent
	if err := doc.DataTo(&agent); err != nil {
		return nil, fmt.Errorf("failed to parse agent data: %w", err)
	}

	return &agent, nil
}

// GetAgentByPhoneNumber retrieves an agent by phone number
func (s *FirestoreService) GetAgentByPhoneNumber(ctx context.Context, phoneNumber string) (*models.Agent, error) {
	iter := s.client.Collection(s.agentsCollection).
		Where("phone_number", "==", phoneNumber).
		Limit(1).
		Documents(ctx)

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, fmt.Errorf("agent not found for phone number: %s", phoneNumber)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query agent: %w", err)
	}

	var agent models.Agent
	if err := doc.DataTo(&agent); err != nil {
		return nil, fmt.Errorf("failed to parse agent data: %w", err)
	}

	return &agent, nil
}

// ListAgents retrieves all agents for a user
func (s *FirestoreService) ListAgents(ctx context.Context, ownerID string) ([]*models.Agent, error) {
	var agents []*models.Agent

	query := s.client.Collection(s.agentsCollection)
	if ownerID != "" {
		query = query.Where("owner_id", "==", ownerID)
	}

	iter := query.Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate agents: %w", err)
		}

		var agent models.Agent
		if err := doc.DataTo(&agent); err != nil {
			return nil, fmt.Errorf("failed to parse agent data: %w", err)
		}

		agents = append(agents, &agent)
	}

	return agents, nil
}

// UpdateAgent updates an existing agent
func (s *FirestoreService) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	agent.UpdatedAt = time.Now()

	_, err := s.client.Collection(s.agentsCollection).Doc(agent.ID).Set(ctx, agent)
	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	return nil
}

// DeleteAgent deletes an agent
func (s *FirestoreService) DeleteAgent(ctx context.Context, agentID string) error {
	_, err := s.client.Collection(s.agentsCollection).Doc(agentID).Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	return nil
}

// Session Operations

// CreateSession creates a new call session
func (s *FirestoreService) CreateSession(ctx context.Context, session *models.Session) error {
	session.CreatedAt = time.Now()
	session.UpdatedAt = time.Now()

	_, err := s.client.Collection(s.sessionsCollection).Doc(session.ID).Set(ctx, session)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// GetSession retrieves a session by ID
func (s *FirestoreService) GetSession(ctx context.Context, sessionID string) (*models.Session, error) {
	doc, err := s.client.Collection(s.sessionsCollection).Doc(sessionID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session models.Session
	if err := doc.DataTo(&session); err != nil {
		return nil, fmt.Errorf("failed to parse session data: %w", err)
	}

	return &session, nil
}

// GetSessionByCallID retrieves a session by call ID
func (s *FirestoreService) GetSessionByCallID(ctx context.Context, callID string) (*models.Session, error) {
	iter := s.client.Collection(s.sessionsCollection).
		Where("call_id", "==", callID).
		Limit(1).
		Documents(ctx)

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, fmt.Errorf("session not found for call ID: %s", callID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	var session models.Session
	if err := doc.DataTo(&session); err != nil {
		return nil, fmt.Errorf("failed to parse session data: %w", err)
	}

	return &session, nil
}

// UpdateSession updates an existing session
func (s *FirestoreService) UpdateSession(ctx context.Context, session *models.Session) error {
	session.UpdatedAt = time.Now()

	_, err := s.client.Collection(s.sessionsCollection).Doc(session.ID).Set(ctx, session)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// AddMessageToSession adds a message to a session's conversation history
func (s *FirestoreService) AddMessageToSession(ctx context.Context, sessionID string, message models.Message) error {
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	message.Timestamp = time.Now()
	session.ConversationHistory = append(session.ConversationHistory, message)
	session.UpdatedAt = time.Now()

	_, err = s.client.Collection(s.sessionsCollection).Doc(sessionID).Set(ctx, session)
	if err != nil {
		return fmt.Errorf("failed to add message to session: %w", err)
	}

	return nil
}

// DeleteSession deletes a session
func (s *FirestoreService) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := s.client.Collection(s.sessionsCollection).Doc(sessionID).Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// DeleteExpiredSessions deletes sessions older than the specified duration
func (s *FirestoreService) DeleteExpiredSessions(ctx context.Context, olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)

	iter := s.client.Collection(s.sessionsCollection).
		Where("updated_at", "<", cutoffTime).
		Documents(ctx)
	defer iter.Stop()

	batch := s.client.Batch()
	count := 0

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to iterate sessions: %w", err)
		}

		batch.Delete(doc.Ref)
		count++

		// Firestore batch limit is 500
		if count >= 500 {
			if _, err := batch.Commit(ctx); err != nil {
				return fmt.Errorf("failed to commit batch delete: %w", err)
			}
			batch = s.client.Batch()
			count = 0
		}
	}

	if count > 0 {
		if _, err := batch.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit final batch delete: %w", err)
		}
	}

	return nil
}

// User Operations

// CreateUser creates a new user
func (s *FirestoreService) CreateUser(ctx context.Context, user *models.User) error {
	user.CreatedAt = time.Now()
	user.LastLoginAt = time.Now()

	_, err := s.client.Collection("users").Doc(user.ID).Set(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUser retrieves a user by ID
func (s *FirestoreService) GetUser(ctx context.Context, userID string) (*models.User, error) {
	doc, err := s.client.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, fmt.Errorf("failed to parse user data: %w", err)
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (s *FirestoreService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	iter := s.client.Collection("users").
		Where("email", "==", email).
		Limit(1).
		Documents(ctx)

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, fmt.Errorf("user not found for email: %s", email)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, fmt.Errorf("failed to parse user data: %w", err)
	}

	return &user, nil
}

// UpdateUser updates an existing user
func (s *FirestoreService) UpdateUser(ctx context.Context, user *models.User) error {
	_, err := s.client.Collection("users").Doc(user.ID).Set(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// UpdateUserLastLogin updates a user's last login time
func (s *FirestoreService) UpdateUserLastLogin(ctx context.Context, userID string) error {
	_, err := s.client.Collection("users").Doc(userID).Update(ctx, []firestore.Update{
		{Path: "last_login_at", Value: time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to update user last login: %w", err)
	}

	return nil
}
