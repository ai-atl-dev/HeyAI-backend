package main

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"cloud.google.com/go/vertexai/genai"
)

// global Gemini client and model
var geminiClient *genai.Client
var geminiModel *genai.GenerativeModel

func main() {
	// Load .env if available
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on system environment variables")
	} else {
		log.Println(".env file loaded successfully")
	}

	log.Println("ELEVENLABS_API_KEY set:", os.Getenv("ELEVENLABS_API_KEY") != "")
	log.Println("ELEVEN_VOICE_ID set:", os.Getenv("ELEVEN_VOICE_ID") != "")

	// initialize Gemini client once
	if err := initGeminiClient(); err != nil {
		log.Fatalf("Failed to initialize Gemini client: %v", err)
	}

	http.HandleFunc("/voice", voiceHandler)
	http.HandleFunc("/speech-result", speechResultHandler)
	http.HandleFunc("/audio", audioHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// initialize the Gemini client once
func initGeminiClient() error {
	projectID := os.Getenv("GCP_PROJECT_ID")
	region := os.Getenv("GCP_REGION")
	if projectID == "" || region == "" {
		return fmt.Errorf("GCP_PROJECT_ID or GCP_REGION not set")
	}

	client, err := genai.NewClient(context.Background(), projectID, region)
	if err != nil {
		return fmt.Errorf("genai.NewClient: %w", err)
	}

	geminiClient = client
	geminiModel = geminiClient.GenerativeModel("gemini-2.5-flash")
	return nil
}

// askGemini generates a response using the reused client
func askGemini(ctx context.Context, question string) (string, error) {
	if geminiClient == nil || geminiModel == nil {
		return "", fmt.Errorf("Gemini client not initialized")
	}

	prompt := fmt.Sprintf("You are a helpful assistant.\nUser: %s\nAssistant:", question)
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	resp, err := geminiModel.GenerateContent(ctxWithTimeout, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("GenerateContent: %w", err)
	}

	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		var out strings.Builder
		for _, part := range resp.Candidates[0].Content.Parts {
			if textPart, ok := part.(genai.Text); ok {
				out.WriteString(string(textPart))
			}
		}
		return out.String(), nil
	}
	return "", fmt.Errorf("no response candidates from Gemini")
}

// Twilio handlers
func voiceHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/xml")
    twiml := `<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Say voice="alice">Hi — welcome. Please ask your question after the beep.</Say>
  <Gather input="speech" action="/speech-result" method="POST" speechTimeout="auto"/>
  <Say>We didn’t get any input. Goodbye.</Say>
</Response>`
    fmt.Fprint(w, twiml)
}

func speechResultHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	userText := r.FormValue("SpeechResult")
	from := r.FormValue("From")
	log.Printf("Received from %s: %s\n", from, userText)

	lower := strings.ToLower(userText)
	if strings.Contains(lower, "hang up") || strings.Contains(lower, "goodbye") {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, `<Response><Say>Okay, goodbye!</Say><Hangup/></Response>`)
		return
	}

	answer, err := askGemini(r.Context(), userText)
	if err != nil {
		log.Printf("Gemini error: %v", err)
		answer = "Sorry, I couldn't generate a response right now."
	}
	answer = html.EscapeString(answer)

	answerURL := urlQueryEscape(answer)

	host := r.Host
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}
	audioURL := fmt.Sprintf("%s/audio?text=%s", host, answerURL)

	w.Header().Set("Content-Type", "text/xml")
	twiml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Play>%s</Play>
  <Gather input="speech" action="/speech-result" method="POST" speechTimeout="auto">
    <Say>What else can I help you with?</Say>
  </Gather>
  <Say>No response detected. Goodbye!</Say>so
</Response>`, audioURL)
	fmt.Fprint(w, twiml)
}

func audioHandler(w http.ResponseWriter, r *http.Request) {
	text := r.URL.Query().Get("text")
	if text == "" {
		http.Error(w, "missing text param", http.StatusBadRequest)
		return
	}

	audioBytes, err := generateElevenLabsAudio(r.Context(), text)
	if err != nil {
		log.Printf("TTS error: %v", err)
		http.Error(w, "TTS generation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(audioBytes)
}

func generateElevenLabsAudio(ctx context.Context, text string) ([]byte, error) {
	apiKey := os.Getenv("ELEVENLABS_API_KEY")
	voiceID := os.Getenv("ELEVEN_VOICE_ID")
	if apiKey == "" || voiceID == "" {
		return nil, fmt.Errorf("ELEVENLABS_API_KEY or ELEVEN_VOICE_ID not set")
	}

	url := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s", voiceID)
	body := fmt.Sprintf(`{"text":%q}`, text)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ElevenLabs API error %d: %s", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

// helper
func urlQueryEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
