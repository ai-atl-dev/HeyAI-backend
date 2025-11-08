<div align="center">
  <img src="assets/HeyAI Logo.jpg" alt="HeyAI Logo" width="200"/>
  
  # HeyAI Backend
  
  <p>Go-based backend service for the AI Voice Agent platform. Handles Twilio voice webhooks, integrates with Vertex AI for conversational AI, and provides REST APIs for the admin console.</p>
</div>

## Architecture Overview

This service acts as the central backend that:
- Receives incoming calls via Twilio Voice API webhooks
- Processes conversations using Vertex AI (Gemini 2.5 Flash)
- Stores call data and analytics in BigQuery
- Manages agent configurations in Firestore
- Provides APIs for the admin console frontend
- Handles OAuth authentication for admin users

## Prerequisites

- Go 1.21 or higher
- GCP account with billing enabled
- Twilio account
- gcloud CLI installed and configured

## Project Structure

```
.
├── cmd/server/main.go              # Application entry point
├── internal/
│   ├── handlers/
│   │   ├── agent.go                # Twilio webhook handler
│   │   ├── admin.go                # Admin console APIs
│   │   └── auth.go                 # OAuth handlers
│   ├── services/
│   │   ├── bigquery.go             # BigQuery integration
│   │   ├── vertexai.go             # Vertex AI integration
│   │   ├── twilio.go               # Twilio Voice API
│   │   └── firestore.go            # Firestore operations
│   ├── middleware/
│   │   └── auth.go                 # Authentication middleware
│   └── models/
│       └── types.go                # Data models
├── configs/config.go               # Configuration management
├── Dockerfile                      # Container definition
└── cloudbuild.yaml                 # GCP Cloud Build config
```

## GCP Setup

### 1. Enable Required APIs

```bash
gcloud services enable run.googleapis.com
gcloud services enable bigquery.googleapis.com
gcloud services enable firestore.googleapis.com
gcloud services enable aiplatform.googleapis.com
gcloud services enable secretmanager.googleapis.com
```

### 2. Create Service Account

```bash
gcloud iam service-accounts create dashboard-backend-sa \
  --display-name="Dashboard Backend Service Account"

gcloud projects add-iam-policy-binding [PROJECT_ID] \
  --member="serviceAccount:dashboard-backend-sa@[PROJECT_ID].iam.gserviceaccount.com" \
  --role="roles/bigquery.dataEditor"

gcloud projects add-iam-policy-binding [PROJECT_ID] \
  --member="serviceAccount:dashboard-backend-sa@[PROJECT_ID].iam.gserviceaccount.com" \
  --role="roles/datastore.user"

gcloud projects add-iam-policy-binding [PROJECT_ID] \
  --member="serviceAccount:dashboard-backend-sa@[PROJECT_ID].iam.gserviceaccount.com" \
  --role="roles/aiplatform.user"
```

### 3. Create BigQuery Dataset

```bash
bq mk --dataset --location=US [PROJECT_ID]:agent_data
```

### 4. Create Firestore Database

```bash
gcloud firestore databases create --region=us-central1
```

### 5. Store Secrets

```bash
echo -n "your-twilio-auth-token" | gcloud secrets create twilio-auth-token --data-file=-
echo -n "your-oauth-client-secret" | gcloud secrets create oauth-client-secret --data-file=-

gcloud secrets add-iam-policy-binding twilio-auth-token \
  --member="serviceAccount:dashboard-backend-sa@[PROJECT_ID].iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

gcloud secrets add-iam-policy-binding oauth-client-secret \
  --member="serviceAccount:dashboard-backend-sa@[PROJECT_ID].iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
```

## Local Development

### 1. Install Dependencies

```bash
go mod download
```

### 2. Set Up Environment Variables

Copy `.env.example` to `.env` and fill in your values:

```bash
cp .env.example .env
```

### 3. Authenticate with GCP

```bash
gcloud auth application-default login
```

### 4. Run the Server

```bash
go run cmd/server/main.go
```

The server will start on `http://localhost:8080`

## API Endpoints

### Voice Agent
- `POST /webhook/voice` - Twilio voice webhook (receives incoming calls)

### Admin Console
- `GET /api/agents` - List all agents
- `POST /api/agents` - Create new agent
- `GET /api/agents/:id` - Get agent details
- `PUT /api/agents/:id` - Update agent
- `DELETE /api/agents/:id` - Delete agent
- `GET /api/usage-history` - Get usage history
- `POST /api/payment` - Process payment
- `GET /api/live-usage` - Stream live usage data (SSE)

### Authentication
- `POST /auth/login` - Initiate OAuth login
- `GET /auth/callback` - OAuth callback handler

## Deployment

### Deploy to Cloud Run

```bash
gcloud run deploy dashboard-backend \
  --source . \
  --platform managed \
  --region us-central1 \
  --service-account dashboard-backend-sa@[PROJECT_ID].iam.gserviceaccount.com \
  --set-env-vars GCP_PROJECT_ID=[PROJECT_ID],GCP_REGION=us-central1,BIGQUERY_DATASET=agent_data \
  --set-secrets TWILIO_AUTH_TOKEN=twilio-auth-token:latest,OAUTH_CLIENT_SECRET=oauth-client-secret:latest \
  --allow-unauthenticated
```

### Configure Twilio Webhook

After deployment, configure your Twilio phone number webhook URL to:
```
https://[YOUR-CLOUD-RUN-URL]/webhook/voice
```

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `GCP_PROJECT_ID` | GCP project ID | Yes |
| `GCP_REGION` | GCP region | Yes |
| `TWILIO_ACCOUNT_SID` | Twilio account SID | Yes |
| `TWILIO_AUTH_TOKEN` | Twilio auth token | Yes |
| `TWILIO_PHONE_NUMBER` | Twilio phone number | Yes |
| `BIGQUERY_DATASET` | BigQuery dataset name | Yes |
| `FIRESTORE_COLLECTION` | Firestore collection name | Yes |
| `OAUTH_CLIENT_ID` | OAuth client ID | Yes |
| `OAUTH_CLIENT_SECRET` | OAuth client secret | Yes |
| `PORT` | Server port | No (default: 8080) |

## Testing

```bash
go test ./...
```

## CI/CD

This project uses Cloud Build for continuous deployment. Push to the main branch triggers automatic deployment to Cloud Run.

## License

See LICENSE file for details.
