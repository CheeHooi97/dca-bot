package bot

import (
	"bot-1/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func sendTelegramMessage(token, message string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	// Use a map for the JSON payload
	payload := map[string]any{
		"chat_id": config.TelegramChatId, // Make sure it's correct
		// "message_thread_id": threadId,              // Can be int64
		"text": message, // Must not be empty
	}

	// Marshal the map into JSON
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error encoding JSON:", err)
		return
	}

	// Send the request
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Println("Error sending message:", err)
		return
	}
	defer resp.Body.Close()

	// Read and print the response
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to send message: %s\nBody: %s\n", resp.Status, string(body))
	}
}
