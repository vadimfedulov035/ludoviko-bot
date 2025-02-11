package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	API = "http://0.0.0.0:8000/api/chat"
	MAX_API_SEND_TRY = 3
)

type Settings struct {
	SystemPrompt string   `json:"system_prompt"`
	ThinkPrompts []string `json:"think_prompts"`
	RatePrompt   string   `json:"rate_prompt"`

	Temperature       float32 `json:"temperature"`
	RepetitionPenalty float32 `json:"repetition_penalty"`
	TopP              float32 `json:"top_p"`
	TopK              int     `json:"top_k"`

	MaxNewTokens      int `json:"max_new_tokens"`
	DynamicTokenShift int `json:"dynamic_token_shift"`
	RateTokens        int `json:"rate_tokens"`

	RespBatchSize  int `json:"resp_batch_size"`
	RateBatchSize  int `json:"rate_batch_size"`
}

type RequestBody struct {
	Dialog   []string `json:"dialog"`
	Settings Settings `json:"settings"`
}

type ResponseBody struct {
	Response string `json:"response"`
}

func newRequestBody(dialog []string, config string) *RequestBody {

	loadSettings := func(config string) Settings {
		var settings Settings
		data, err := os.ReadFile(config)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(data, &settings)
		if err != nil {
			panic(err)
		}
		return settings
	}

	return &RequestBody{
		Dialog:   dialog,
		Settings: loadSettings(config),
	}
}

func sendRequestBody(requestBody *RequestBody) (string, error) {
	// encode request body to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	// make new POST request with JSON data
	req, err := http.NewRequest("POST", API, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	// set HTTP client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// check status; print status code if error
	if resp.StatusCode != http.StatusOK {
		error_msg := "Status code %d"
		return "", fmt.Errorf(error_msg, resp.StatusCode)
	}

	// decode response body
	var responseBody ResponseBody
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		return "", err
	}

	return responseBody.Response, nil
}

// outer handler for request
func Send(dialog []string, config string, chatTitle string) (string, error) {
	// prepare request body
	requestBody := newRequestBody(dialog, config)

	// add chat title to system prompt if space reserved
	prompt := requestBody.Settings.SystemPrompt
	if strings.Contains(prompt, "%s") {
		prompt = fmt.Sprintf(prompt, chatTitle)
	}
	requestBody.Settings.SystemPrompt = prompt

	// send request body
	var text string
	var err error
	for i := range(MAX_API_SEND_TRY) {
		text, err = sendRequestBody(requestBody)
		if err == nil {
			break
		}
		log.Printf("[API] Try %d: %v", i, err)
		time.Sleep(time.Second)
	}

	return text, err
}
