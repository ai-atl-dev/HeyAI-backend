# HeyAI Backend

![Architecture](architecture.png)

Voice-enabled AI assistant accessible via phone call. This backend service orchestrates telephony, speech recognition, AI processing, and text-to-speech synthesis to enable natural conversations with AI through any phone.

## Overview

HeyAI Backend is a Go-based microservice that serves as the orchestration layer between Twilio Voice API, external AI agents, and ElevenLabs text-to-speech. Users can call a phone number, speak their questions, and receive AI-generated responses in natural-sounding voice.

## Architecture

The system follows a multi-tier architecture with the following components:

### Call Flow

1. **User Interaction**: User dials the Twilio phone number and speaks a question
2. **Twilio Voice API**: Receives the call, transcribes speech to text, and forwards to backend
3. **HeyAI Backend (Go)**: Processes the request and orchestrates:
   - Text-to-speech conversion via ElevenLabs API
   - AI response generation via external agent service
   - Authorization and call management
4. **External AI Agents**: Python-based AI service (Sesame AI or 11 Labs) hosted on Cloud Run
5. **Dashboard Backend**: Manages agent connections and analytics
6. **BigQuery**: Stores call logs and analytics data
7. **Admin Console**: Frontend interface for managing agents and viewing analytics

### System Components

```
User Call → Twilio Voice API → HeyAI Backend (Go) → External AI Agent
                                      ↓
                                ElevenLabs TTS
                                      ↓
                                Dashboard Backend
                                      ↓
                                  BigQuery
```

## Technology Stack

### Core Technologies

- **Language**: Go 1.25.4
- **Runtime**: Google Cloud Run (serverless containers)
- **Containerization**: Docker with multi-stage builds
- **CI/CD**: Google Cloud Build

### External Services

- **Telephony**: Twilio Voice API
  - Speech recognition (speech-to-text)
  - Call management and routing
  - TwiML response handling

- **AI Processing**: External Python AI Service
  - Vertex AI hosted Gemini 2.5 Flash
  - Streaming response support
  - Custom agent endpoints

- **Voice Synthesis**: ElevenLabs API
  - Text-to-speech conversion
  - High-quality voice generation
  - MP3 audio streaming

### Google Cloud Platform Services

- **Cloud Run**: Serverless container hosting
- **Artifact Registry**: Container image storage
- **Secret Manager**: Secure credential management
- **Cloud Build**: Automated CI/CD pipeline
- **BigQuery**: Analytics and call data storage (planned)

### Go Dependencies

```
cloud.google.com/go/vertexai v0.15.0
github.com/joho/godotenv v1.5.1
```

## API Endpoints

### POST /voice

Initial Twilio webhook endpoint that handles incoming calls.

**Response**: TwiML XML instructing Twilio to gather speech input

**Example Response**:
```xml
<Response>
  <Say voice="alice">Hi — welcome. Please ask your question after the beep.</Say>
  <Gather input="speech" action="/speech-result" method="POST" speechTimeout="auto"/>
</Response>
```

### POST /speech-result

Processes transcribed speech from Twilio and generates AI responses.

**Request Parameters**:
- `SpeechResult`: Transcribed user speech from Twilio
- `From`: Caller's phone number

**Response**: TwiML XML with audio playback and continuation prompt

**Flow**:
1. Receives transcribed speech from Twilio
2. Forwards question to external AI agent service
3. Generates audio from AI response via ElevenLabs
4. Returns TwiML with audio URL and continuation prompt

### GET /audio

Generates and streams text-to-speech audio.

**Query Parameters**:
- `text`: Text to convert to speech

**Response**: MP3 audio stream (audio/mpeg)

**Implementation**:
- Calls ElevenLabs API with configured voice ID
- Streams MP3 audio directly to caller
- Includes cache control headers

## Configuration

### Environment Variables

Required environment variables:

```bash
# ElevenLabs Configuration
ELEVENLABS_API_KEY=your_elevenlabs_api_key
ELEVEN_VOICE_ID=your_voice_id

# Server Configuration
PORT=8080

# Google Cloud Configuration (for Cloud Run deployment)
GCP_PROJECT_ID=your_project_id
GCP_REGION=us-central1

# External Services
KOOZIE_AGENT_URI=https://your-agent-service.run.app
```

### Secret Management

Secrets are managed via Google Cloud Secret Manager in production:
- `ELEVENLABS_API_KEY`: ElevenLabs API authentication
- `ELEVEN_VOICE_ID`: Voice model identifier

## Deployment

### Local Development

1. Install Go 1.25 or higher
2. Clone the repository
3. Copy `.env.example` to `.env` and configure variables
4. Install dependencies:
   ```bash
   go mod download
   ```
5. Run the server:
   ```bash
   go run main.go
   ```
6. Expose local server with ngrok:
   ```bash
   ngrok http 8080
   ```
7. Configure Twilio webhook URL to ngrok endpoint

### Production Deployment

The service is deployed to Google Cloud Run via Cloud Build:

1. **Build**: Multi-stage Docker build creates optimized binary
2. **Push**: Image pushed to Artifact Registry
3. **Deploy**: Cloud Run service updated with new image

**Deployment Command**:
```bash
gcloud builds submit --config cloudbuild.yaml
```

**Cloud Run Configuration**:
- Platform: Managed
- Region: us-central1
- Port: 8080
- Authentication: Allow unauthenticated (for Twilio webhooks)
- Secrets: Injected from Secret Manager

## Features

### Implemented

- Natural language conversation via phone call
- Multi-turn conversation support with context
- Real-time speech-to-text via Twilio
- AI response generation via external agent service
- High-quality text-to-speech via ElevenLabs
- Graceful error handling and fallbacks
- Conversation termination on user request
- Structured logging for debugging
- Secure credential management
- Containerized deployment
- Auto-scaling serverless infrastructure

### Planned

- Call recording and transcription storage
- BigQuery integration for analytics
- Multi-language support
- Custom voice selection per agent
- WebSocket streaming for reduced latency
- Admin dashboard integration
- Usage metrics and monitoring

## Integration Points

### External AI Agent Service

The backend communicates with a Python-based AI service that handles:
- Gemini 2.5 Flash model inference
- Streaming response generation
- Context management
- Agent-specific logic

**API Contract**:
```json
POST /chat
{
  "message": "user question"
}

Response: Server-Sent Events (SSE) stream
data: {"text": "response chunk"}
```

### Twilio Integration

Twilio webhooks are configured to point to:
- `/voice` - Initial call handling
- `/speech-result` - Speech processing

### Dashboard Backend

Planned integration for:
- Call analytics
- Agent management
- Usage tracking
- BigQuery data storage

## Development

### Project Structure

```
HeyAI-backend/
├── main.go              # Main application entry point
├── go.mod               # Go module dependencies
├── go.sum               # Dependency checksums
├── Dockerfile           # Multi-stage container build
├── cloudbuild.yaml      # Cloud Build CI/CD configuration
├── .env                 # Local environment variables
├── .gitignore           # Git ignore rules
└── README.md            # This file
```

### Key Functions

- `voiceHandler`: Handles initial Twilio call webhook
- `speechResultHandler`: Processes speech and generates responses
- `audioHandler`: Streams TTS audio
- `askPythonAI`: Communicates with external AI service
- `generateElevenLabsAudio`: Generates speech from text

## Performance

- Response latency: Sub-3 seconds from question to audio playback
- Concurrent request handling via Go's native concurrency
- Stateless design for horizontal scaling
- Optimized Docker image with distroless base

## Security

- Secrets stored in Google Cloud Secret Manager
- Non-root container execution
- HTTPS-only communication
- Environment variable validation
- Input sanitization for TwiML generation

## License

MIT License