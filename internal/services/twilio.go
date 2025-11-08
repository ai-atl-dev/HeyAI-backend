package services

import (
	"encoding/xml"
	"fmt"

	"github.com/ai-atl-dev/HeyAI-backend/internal/models"
	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

// TwilioService handles Twilio API interactions
type TwilioService struct {
	client      *twilio.RestClient
	phoneNumber string
}

// NewTwilioService creates a new Twilio service
func NewTwilioService(config *models.Config) *TwilioService {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: config.TwilioAccountSID,
		Password: config.TwilioAuthToken,
	})

	return &TwilioService{
		client:      client,
		phoneNumber: config.TwilioPhoneNumber,
	}
}

// GenerateTwiMLResponse generates a TwiML response
func (s *TwilioService) GenerateTwiMLResponse(say string, gather bool, gatherAction string) (string, error) {
	response := models.TwilioResponse{}

	if gather {
		response.Gather = &models.Gather{
			Input:   "speech",
			Action:  gatherAction,
			Method:  "POST",
			Timeout: 3,
			Say: &models.Say{
				Voice:    "Polly.Joanna",
				Language: "en-US",
				Text:     say,
			},
		}
	} else {
		response.Say = []models.Say{
			{
				Voice:    "Polly.Joanna",
				Language: "en-US",
				Text:     say,
			},
		}
	}

	xmlData, err := xml.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal TwiML: %w", err)
	}

	return xml.Header + string(xmlData), nil
}

// GenerateHangupResponse generates a TwiML hangup response
func (s *TwilioService) GenerateHangupResponse(say string) (string, error) {
	response := models.TwilioResponse{
		Say: []models.Say{
			{
				Voice:    "Polly.Joanna",
				Language: "en-US",
				Text:     say,
			},
		},
		Hangup: &models.Hangup{},
	}

	xmlData, err := xml.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal TwiML: %w", err)
	}

	return xml.Header + string(xmlData), nil
}

// MakeCall initiates an outbound call
func (s *TwilioService) MakeCall(to, callbackURL string) (string, error) {
	params := &twilioApi.CreateCallParams{}
	params.SetTo(to)
	params.SetFrom(s.phoneNumber)
	params.SetUrl(callbackURL)

	resp, err := s.client.Api.CreateCall(params)
	if err != nil {
		return "", fmt.Errorf("failed to make call: %w", err)
	}

	return *resp.Sid, nil
}

// GetCallDetails retrieves call details
func (s *TwilioService) GetCallDetails(callSID string) (*twilioApi.ApiV2010Call, error) {
	call, err := s.client.Api.FetchCall(callSID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch call: %w", err)
	}

	return call, nil
}

// EndCall terminates an active call
func (s *TwilioService) EndCall(callSID string) error {
	params := &twilioApi.UpdateCallParams{}
	status := "completed"
	params.SetStatus(status)

	_, err := s.client.Api.UpdateCall(callSID, params)
	if err != nil {
		return fmt.Errorf("failed to end call: %w", err)
	}

	return nil
}
