package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"cloud.google.com/go/vertexai/genai"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Thread-safe WebSocket writer
type SafeWS struct {
	ws   *websocket.Conn
	lock sync.Mutex
}

func (s *SafeWS) WriteJSON(v interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.ws.WriteJSON(v)
}

// Store active WebSocket connections by call SID
var activeStreams = struct {
	sync.RWMutex
	streams map[string]*SafeWS
}{streams: make(map[string]*SafeWS)}

func main() {
	_ = godotenv.Load()

	http.HandleFunc("/voice", voiceHandler)
	http.HandleFunc("/process-speech", processSpeechHandler)
	http.HandleFunc("/stream", streamHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Server listening on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Twilio voice endpoint - initial call
func voiceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/xml")
	resp := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Connect>
    <Stream url="wss://%s/stream"/>
  </Connect>
  <Gather input="speech" action="https://%s/process-speech" speechTimeout="auto" language="en-US">
    <Say voice="alice">Hi, welcome! Please ask your question.</Say>
  </Gather>
  <Say>We didn't get any input. Goodbye.</Say>
</Response>`, r.Host, r.Host)
	w.Write([]byte(resp))
}

// Process speech recognition result from Twilio
func processSpeechHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	speechResult := r.FormValue("SpeechResult")
	callSid := r.FormValue("CallSid")

	log.Printf("🗣️ Speech received: %s (Call: %s)", speechResult, callSid)

	if speechResult == "" {
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Say>I didn't catch that. Goodbye.</Say>
</Response>`))
		return
	}

	// Get the WebSocket connection for this call
	activeStreams.RLock()
	safeWS, exists := activeStreams.streams[callSid]
	activeStreams.RUnlock()

	if !exists || safeWS == nil {
		log.Printf("⚠️ No active stream found for call %s", callSid)
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Say>Sorry, there was a connection error. Goodbye.</Say>
</Response>`))
		return
	}

	// Process with Gemini and stream to TTS
	ctx := context.Background()
	go func() {
		if err := streamGeminiAndTTS(ctx, safeWS, speechResult); err != nil {
			log.Printf("Error processing speech: %v", err)
		}
	}()

	// Return TwiML to keep the call open while streaming
	w.Header().Set("Content-Type", "text/xml")
	resp := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
  <Pause length="15"/>
  <Gather input="speech" action="https://%s/process-speech" speechTimeout="auto" language="en-US">
    <Say voice="alice">Ask me another question, or stay silent to end the call.</Say>
  </Gather>
  <Say>Thanks for calling. Goodbye!</Say>
</Response>`, r.Host)
	w.Write([]byte(resp))
}

// WebSocket streaming endpoint
func streamHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer ws.Close()

	safeWS := &SafeWS{ws: ws}
	var callSid string

	log.Println("🔌 WebSocket connection established")

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			log.Println("❌ WS read error:", err)
			// Remove from active streams
			if callSid != "" {
				activeStreams.Lock()
				delete(activeStreams.streams, callSid)
				activeStreams.Unlock()
				log.Printf("🗑️ Removed stream for call %s", callSid)
			}
			return
		}

		var evt map[string]interface{}
		if err := json.Unmarshal(msg, &evt); err != nil {
			continue
		}

		eventType, _ := evt["event"].(string)

		switch eventType {
		case "connected":
			log.Println("✅ Twilio connected event received")

		case "start":
			log.Println("🎬 Stream started")
			startObj, ok := evt["start"].(map[string]interface{})
			if ok {
				callSid, _ = startObj["callSid"].(string)
				streamSid, _ := startObj["streamSid"].(string)
				log.Printf("📞 Call SID: %s, Stream SID: %s", callSid, streamSid)

				// Store this connection
				activeStreams.Lock()
				activeStreams.streams[callSid] = safeWS
				activeStreams.Unlock()
			}

		case "media":
			// We don't process media in this approach since Twilio handles speech recognition

		case "stop":
			log.Println("🛑 Stream stopped")
			if callSid != "" {
				activeStreams.Lock()
				delete(activeStreams.streams, callSid)
				activeStreams.Unlock()
			}
			return
		}
	}
}

// Gemini streaming + ElevenLabs TTS
func streamGeminiAndTTS(ctx context.Context, safeWS *SafeWS, prompt string) error {
	projectID := os.Getenv("GCP_PROJECT_ID")
	region := os.Getenv("GCP_REGION")
	apiKey := os.Getenv("ELEVENLABS_API_KEY")
	voiceID := os.Getenv("ELEVEN_VOICE_ID")

	if projectID == "" || region == "" || apiKey == "" || voiceID == "" {
		return fmt.Errorf("missing environment variables")
	}

	client, err := genai.NewClient(ctx, projectID, region)
	if err != nil {
		return err
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.0-flash-exp")

	iter := model.GenerateContentStream(ctx,
		genai.Text(fmt.Sprintf("You are a concise helpful assistant. Give a brief answer in 2-3 sentences: %s", prompt)),
	)

	log.Println("💡 Gemini streaming started")

	var sentenceBuffer strings.Builder

	for {
		resp, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Gemini iterator error: %w", err)
		}

		for _, c := range resp.Candidates {
			for _, part := range c.Content.Parts {
				if t, ok := part.(genai.Text); ok {
					textChunk := string(t)
					sentenceBuffer.WriteString(textChunk)

					// Check if we have a complete sentence
					text := sentenceBuffer.String()
					if strings.Contains(text, ".") || strings.Contains(text, "!") || strings.Contains(text, "?") {
						// Find the last sentence boundary
						lastPeriod := strings.LastIndex(text, ".")
						lastExclaim := strings.LastIndex(text, "!")
						lastQuestion := strings.LastIndex(text, "?")

						lastBoundary := lastPeriod
						if lastExclaim > lastBoundary {
							lastBoundary = lastExclaim
						}
						if lastQuestion > lastBoundary {
							lastBoundary = lastQuestion
						}

						if lastBoundary > 0 {
							sentence := strings.TrimSpace(text[:lastBoundary+1])
							remainder := text[lastBoundary+1:]

							log.Printf("💬 Sending to TTS: %s", sentence)
							if err := streamToElevenLabsAndTwilio(ctx, safeWS, apiKey, voiceID, sentence); err != nil {
								log.Printf("TTS error: %v", err)
							}

							sentenceBuffer.Reset()
							sentenceBuffer.WriteString(remainder)
						}
					}
				}
			}
		}
	}

	// Send any remaining text
	if sentenceBuffer.Len() > 0 {
		remaining := strings.TrimSpace(sentenceBuffer.String())
		if remaining != "" {
			log.Printf("💬 Sending remaining to TTS: %s", remaining)
			if err := streamToElevenLabsAndTwilio(ctx, safeWS, apiKey, voiceID, remaining); err != nil {
				log.Printf("TTS error: %v", err)
			}
		}
	}

	log.Println("✅ Gemini streaming completed")
	return nil
}

// Convert text -> ElevenLabs TTS -> stream to Twilio
func streamToElevenLabsAndTwilio(ctx context.Context, safeWS *SafeWS, apiKey, voiceID, text string) error {
	// Use ElevenLabs streaming with proper format for Twilio
	reqBody := map[string]interface{}{
		"text":          text,
		"model_id":      "eleven_turbo_v2_5",
		"output_format": "ulaw_8000", // Twilio's native format
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s/stream", voiceID),
		bytes.NewBuffer(bodyBytes),
	)
	if err != nil {
		return fmt.Errorf("TTS request build error: %w", err)
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("TTS request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TTS API error: %d - %s", resp.StatusCode, string(body))
	}

	log.Printf("🔊 Streaming audio to Twilio...")

	// Read the entire audio stream
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read audio: %w", err)
	}

	// Send in chunks of 160 bytes (20ms at 8kHz)
	chunkSize := 160
	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		if end > len(audioData) {
			end = len(audioData)
		}

		chunk := audioData[i:end]
		payload := base64.StdEncoding.EncodeToString(chunk)

		msg := map[string]interface{}{
			"event": "media",
			"media": map[string]string{
				"payload": payload,
			},
		}

		if err := safeWS.WriteJSON(msg); err != nil {
			return fmt.Errorf("WebSocket write error: %w", err)
		}
	}

	log.Println("✅ Audio streaming completed")
	return nil
}