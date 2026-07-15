// Package router makes requests to local AI
package router

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// aiResponse used for decode response from AI
type aiResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Context  []int  `json:"context"`
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
			Stream: true,
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
func (r Router) Generate(text string, userContext []int, yield func(string)) ([]int, error) {
	const op = "router.Generate"

	r.reqBody.Prompt = text
	r.reqBody.Context = userContext
	r.reqBody.Stream = true

	jsonData, err := json.Marshal(r.reqBody)
	if err != nil {
		return nil, fmt.Errorf("%s:json.Marshal: %w", op, err)
	}
	jsonReader := bytes.NewReader(jsonData)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, jsonReader)
	if err != nil {
		return nil, fmt.Errorf("%s: http.NewRequestWithContext: %w", op, err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: client.Do: %w", op, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: bad status code: %d", op, res.StatusCode)
	}

	var lastContext []int
	reader := bufio.NewReader(res.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("%s: reader.ReadBytes: %w", op, err)
		}
		aiRes := aiResponse{}
		if err := json.Unmarshal(line, &aiRes); err != nil {
			return nil, fmt.Errorf("%s: json.Unmarshal: %w", op, err)
		}
		lastContext = aiRes.Context
		yield(aiRes.Response)
	}

	return lastContext, nil
}
