package main

import (
	"bytes"
	"context"
	"encoding/json"
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
)

// ----- CONFIG -----
var pythonAIURL = "https://koozie-agent-service-127756525541.us-central1.run.app/chat" // replace with your friend's Python server

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	} else {
		log.Println(".env file loaded successfully")
	}

	log.Println("ELEVENLABS_API_KEY set:", os.Getenv("ELEVENLABS_API_KEY") != "")
	log.Println("ELEVEN_VOICE_ID set:", os.Getenv("ELEVEN_VOICE_ID") != "")

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

// ----- TWILIO HANDLERS -----
func voiceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	twiml := `<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Say voice="alice">Hi — welcome. Please ask your question after the beep.</Say>
  <Gather input="speech" action="/speech-result" method="POST" speechTimeout="auto"/>
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

	// Ask your Python AI server
	answer, err := askPythonAI(userText)
	if err != nil {
		log.Printf("Python AI error: %v", err)
		answer = "Sorry, I couldn't generate a response right now."
	}

	answer = html.EscapeString(answer)
	audioURL := fmt.Sprintf("%s/audio?text=%s", ensureHTTPS(r.Host), urlQueryEscape(answer))

	w.Header().Set("Content-Type", "text/xml")
	// Loop with <Gather> + <Redirect> for multiple Q&A cycles
	twiml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Play>%s</Play>
  <Gather input="speech" action="/speech-result" method="POST" speechTimeout="auto">
    <Say>What else can I help you with?</Say>
  </Gather>
  <Redirect>/speech-result</Redirect>
</Response>`, audioURL)
	fmt.Fprint(w, twiml)
}

// ----- PYTHON AI CALL (synchronous) -----
func askPythonAI(question string) (string, error) {
	payload := fmt.Sprintf(`{"message":%q}`, question)
	req, _ := http.NewRequest("POST", pythonAIURL, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Python API returned status %d", resp.StatusCode)
	}

	var fullText strings.Builder
	buf := make([]byte, 4096)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			// split by lines
			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "data: ") {
					dataStr := line[6:]
					var chunk map[string]interface{}
					if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
						log.Printf("Failed to parse chunk: %v", err)
						continue
					}
					if text, ok := chunk["text"].(string); ok {
						fullText.WriteString(text)
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
	}

	return fullText.String(), nil
}


// ----- ELEVENLABS TTS -----
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

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(body))
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

// ----- HELPERS -----
func urlQueryEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func ensureHTTPS(host string) string {
	if strings.HasPrefix(host, "http") {
		return host
	}
	return "https://" + host
}
