package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"os"
	"log"
)


const API = "http://127.0.0.1:8000/api/chat"


type Settings struct {
	SystemPrompt      string    `json:"system_prompt"`
	ThinkPrompts      []string  `json:"think_prompts"`
	RatePrompt        string    `json:"rate_prompt"`

	Temperature       float32   `json:"temperature"`
	RepetitionPenalty float32   `json:"repetition_penalty"`
	TopP              float32   `json:"top_p"`
	TopK              int       `json:"top_k"`

	MaxNewTokens      int       `json:"max_new_tokens"`
	DynamicTokenShift int       `json:"dynamic_token_shift"`
	RateTokens        int       `json:"rate_tokens"`

	BatchSize         int       `json:"batch_size"`
	RateNum           int       `json:"rate_num"`
}


type RequestBody struct {
	Dialog   []string  `json:"dialog"`
	Settings Settings  `json:"settings"`
}


type ResponseBody struct {
	Response string `json:"response"`
}


func loadSettings(conf string) Settings {
    var settings Settings
    data, err := os.ReadFile(conf)
    if err != nil {
        panic(err)
    }
	err = json.Unmarshal(data, &settings)
	if err != nil {
		panic(err)
	}
    return settings
}


func newRequestBody(dialog []string, conf string) *RequestBody {
	return &RequestBody{
		Dialog:   dialog,
		Settings: loadSettings(conf), 
	}
}


func sendRequestBody(requestBody *RequestBody) (string, error) {
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", API, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		error_msg := "Status code %d"
		return "", fmt.Errorf(error_msg, resp.StatusCode)
	}

	var responseBody ResponseBody
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		return "", err
	}

	return responseBody.Response, nil
}


func Send(dialog []string, config string, title string) string {
	// prepare request body
    requestBody := newRequestBody(dialog, config)

	// add chat title to prompt if space reserved
	if title == "" {
		title = "privata interparolo"
	}
	prompt := requestBody.Settings.SystemPrompt
	if strings.Contains(prompt, "%s") {
		prompt = fmt.Sprintf(prompt, title)
	}
    requestBody.Settings.SystemPrompt = prompt

	// send request body
    text, err := sendRequestBody(requestBody)
	if err != nil {
		log.Printf("[API] Sending: %v", err)
	}

	return text
}
