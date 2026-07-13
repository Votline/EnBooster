// Package router makes requests to local AI
package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"aisrv/internal/utils"
)

// option contains AI model options.
type option struct {
	Temperature   float64 `json:"temperature"`
	NumPredicts   int     `json:"num_predict"`
	TopP          float64 `json:"top_p"`
	RepeatPenalty float64 `json:"repeat_penalty"`
}

// requestBody used for HTTP request body to AI.
type requestBody struct {
	Model   string `json:"model"`
	System  string `json:"system"`
	Prompt  string `json:"prompt"`
	Stream  bool   `json:"stream"`
	Context []int  `json:"context"`
	Options option `json:"options"`
}

// aiRes used for decode response from AI
var aiRes struct {
	Response string `json:"response"`
	Message  struct {
		Content string `json:"content"`
	} `json:"message"`
	Done    bool  `json:"done"`
	Context []int `json:"context"`
}

// Router contains all needed fields to call AI
type Router struct {
	timeout int
	url     string
	client  *http.Client
	reqBody requestBody
}

func NewRouter() *Router {
	model := os.Getenv("AI_MODEL")
	system := os.Getenv("AI_SYSTEM")
	timeout := utils.GetEnvInt(os.Getenv("AI_TIMEOUT"), 60) * int(time.Second)

	url := fmt.Sprintf("http://%s:%s/api/generate", os.Getenv("AI_HOST"), os.Getenv("AI_PORT"))

	temp := utils.GetEnvFloat(os.Getenv("AI_TEMP"), 0.7)
	numPredicts := utils.GetEnvInt(os.Getenv("AI_NUM_PREDICTS"), 512)
	topP := utils.GetEnvFloat(os.Getenv("AI_TOP_P"), 0.95)
	repeatPenalty := utils.GetEnvFloat(os.Getenv("AI_REPEAT_PENALTY"), 1.1)

	return &Router{
		reqBody: requestBody{
			Model:  model,
			System: system,
			Stream: false,
			Options: option{
				Temperature:   temp,
				NumPredicts:   numPredicts,
				TopP:          topP,
				RepeatPenalty: repeatPenalty,
			},
		},
		url:     url,
		timeout: timeout,
		client:  http.DefaultClient,
	}
}

// Generate generates text from AI.
func (r Router) Generate(text string, userContext []int) (string, []int, error) {
	const op = "router.Generate"

	r.reqBody.Prompt = text
	r.reqBody.Context = userContext

	jsonData, err := json.Marshal(r.reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("%s:json.Marshal: %w", op, err)
	}
	jsonReader := bytes.NewReader(jsonData)

	res, err := r.client.Post(r.url, "application/json", jsonReader)
	if err != nil {
		return "", nil, fmt.Errorf("%s: http.Post: %w", op, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("%s: not StatusOK: %d", op, res.StatusCode)
	}

	if err := json.NewDecoder(res.Body).Decode(&aiRes); err != nil {
		return "", nil, fmt.Errorf("%s: json.Decode: %w", op, err)
	}

	resText := aiRes.Response
	if resText == "" {
		resText = aiRes.Message.Content
	}

	return resText, aiRes.Context, nil
}
