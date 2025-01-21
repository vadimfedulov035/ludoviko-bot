package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)


const API = "http://127.0.0.1:8000/api/chat"


type UserData struct {
	User              string    `json:"user"`
	Dialog            string    `json:"dialog"`
	Order             string    `json:"order"`
}


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

	ProbeNum          int       `json:"probe_num"`
}


type RequestBody struct {
	UserData          UserData  `json:"user_data"`
	Settings          Settings  `json:"settings"`
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


func NewRequestBody(user string, dialog string, conf string, order string) *RequestBody {

	userData := UserData{
		User:   user,
		Dialog: dialog,
		Order:  order,
	}

	settings := loadSettings(conf)

	requestBody := &RequestBody{
		UserData: userData,
		Settings: settings,  
	}

	return requestBody
}


func SendToAPI(requestBody *RequestBody) (string, error) {

	// unmarshal request body
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	// make request
	req, err := http.NewRequest("POST", API, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	// send request and get response
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// check response status
	if resp.StatusCode != http.StatusOK {
		error_msg := "[API] Status code: %d"
		return "", fmt.Errorf(error_msg, resp.StatusCode)
	}

	// get response
	var responseBody ResponseBody
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	if err != nil {
		return "", err
	}

	return responseBody.Response, nil
}
