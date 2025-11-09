package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

var (
	pythonAIURL  string
	elevenAPIKey string
	elevenVoiceID string
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	} else {
		log.Println(".env file loaded successfully")
	}

	pythonAIURL = "https://koozie-agent-service-127756525541.us-central1.run.app/chat"
	elevenAPIKey = os.Getenv("ELEVENLABS_API_KEY")
	elevenVoiceID = os.Getenv("ELEVEN_VOICE_ID")

	if elevenAPIKey == "" || elevenVoiceID == "" {
		log.Fatal("ELEVENLABS_API_KEY or ELEVEN_VOICE_ID not set in environment or .env")
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

// --- (rest of your handlers remain unchanged) ---


// Voice entrypoint
func voiceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	twiml := `<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Say voice="alice">Welcome. Please ask your question after the beep.</Say>
  <Gather input="speech" action="/speech-result" method="POST" speechTimeout="auto"/>
  <Say>We didn’t get any input. Goodbye.</Say>
</Response>`
	fmt.Fprint(w, twiml)
}

// Handles Twilio speech results
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

	audioURL := fmt.Sprintf("https://%s/audio?question=%s", r.Host, urlQueryEscape(userText))
	w.Header().Set("Content-Type", "text/xml")
//     twiml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
// <Response>
//   <Play>%s</Play>
//   <Gather input="speech" action="/speech-result" method="POST" speechTimeout="auto"/>
//   <Say>No response detected. Goodbye.</Say>
// </Response>`, audioURL)
	twiml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Play>%s</Play>
  <Gather input="speech" action="/speech-result" method="POST" speechTimeout="auto">
    <Say>go ahead</Say>
  </Gather>
  <Say>No response detected. Goodbye.</Say>
</Response>`, audioURL)
	fmt.Fprint(w, twiml)
}

// Streams TTS for the AI response
func audioHandler(w http.ResponseWriter, r *http.Request) {
	question := r.URL.Query().Get("question")
	if question == "" {
		http.Error(w, "missing question param", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Call Python SSE endpoint
	reqBody := fmt.Sprintf(`{"message":%q}`, question)
	req, _ := http.NewRequestWithContext(ctx, "POST", pythonAIURL, strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error calling Python AI: %v", err)
		http.Error(w, "AI error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		log.Printf("SSE chunk: %s", data)

		text := extractTextFromSSE(data)
		if text == "" {
			continue
		}

		audioChunk, err := generateElevenLabsAudio(ctx, text)
		if err != nil {
			log.Printf("TTS error: %v", err)
			continue
		}

		_, _ = w.Write(audioChunk)
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading SSE: %v", err)
	}
}

// Extract "text" field from SSE JSON
func extractTextFromSSE(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || line == "data:" {
		return ""
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		log.Printf("JSON parse error: %v", err)
		return ""
	}
	if t, ok := parsed["text"].(string); ok {
		return strings.TrimSpace(t)
	}
	return ""
}

// Generate ElevenLabs TTS audio
func generateElevenLabsAudio(ctx context.Context, text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("empty text, skipping TTS call")
	}

	url := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s", elevenVoiceID)
	body := fmt.Sprintf(`{"text":%q}`, text)

	log.Printf("Calling ElevenLabs with voiceID=%q text=%q", elevenVoiceID, text)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(body))
	req.Header.Set("xi-api-key", elevenAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	resp, err := http.DefaultClient.Do(req)
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

// URL escape helper
func urlQueryEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
