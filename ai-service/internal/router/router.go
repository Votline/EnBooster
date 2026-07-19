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
	"os/exec"
	"strconv"
	"strings"
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

type textToText struct {
	url      string
	defaulsp string
	client   *http.Client
	reqBody  requestBody
}

type textToSpeech struct {
	path string
	args string
}

// Router contains all needed fields to call AI
type Router struct {
	timeout int
	ttt     textToText
	tts     textToSpeech
}

func NewRouter() *Router {
	model := os.Getenv("AI_MODEL")
	system := os.Getenv("AI_SYSTEM")
	timeout := utils.GetEnvInt(os.Getenv("AI_TIMEOUT"), 60) * int(time.Second)

	url := fmt.Sprintf("http://%s:%s/api/generate", os.Getenv("AI_HOST"), os.Getenv("AI_PORT"))
	defaultSystemPrompt := os.Getenv("AI_DEFAULT_SYSTEM_PROMPT")

	temp := utils.GetEnvFloat(os.Getenv("AI_TEMP"), 0.7)
	numPredicts := utils.GetEnvInt(os.Getenv("AI_NUM_PREDICTS"), 512)
	topP := utils.GetEnvFloat(os.Getenv("AI_TOP_P"), 0.95)
	repeatPenalty := utils.GetEnvFloat(os.Getenv("AI_REPEAT_PENALTY"), 1.1)

	pathTTS := os.Getenv("TTS_PATH")
	modelTTS := os.Getenv("TTS_MODEL")
	sampleRate := utils.GetEnvInt(os.Getenv("TTS_SAMPLE_RATE"), 16000)
	args := " -p " + modelTTS + " -R " + strconv.Itoa(sampleRate) + " -o /dev/stdout"

	return &Router{
		ttt: textToText{
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
			url:      url,
			client:   http.DefaultClient,
			defaulsp: defaultSystemPrompt,
		},
		tts: textToSpeech{
			path: pathTTS,
			args: args,
		},
		timeout: timeout,
	}
}

// GenerateText generates text from AI.
func (r Router) GenerateText(prompt, systemPrompt string, userContext []int, yield func(string)) ([]int, error) {
	const op = "router.GenerateText"

	r.ttt.reqBody.Prompt = prompt
	r.ttt.reqBody.Context = userContext
	r.ttt.reqBody.Stream = true

	r.ttt.reqBody.System = systemPrompt
	switch systemPrompt {
	case "default":
		r.ttt.reqBody.System = r.ttt.defaulsp
	case "nop":
		r.ttt.reqBody.System = ""
	}

	jsonData, err := json.Marshal(r.ttt.reqBody)
	if err != nil {
		return nil, fmt.Errorf("%s:json.Marshal: %w", op, err)
	}
	jsonReader := bytes.NewReader(jsonData)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.ttt.url, jsonReader)
	if err != nil {
		return nil, fmt.Errorf("%s: http.NewRequestWithContext: %w", op, err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := r.ttt.client.Do(req)
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
			return nil, fmt.Errorf("%s: reader.ttt.ReadBytes: %w", op, err)
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

// GenerateAudio calls script to generate audio from text.
func (r Router) GenerateAudio(text string, buf *[]byte, ctx context.Context) error {
	const op = "router.GenerateAudio"

	cmd := exec.CommandContext(ctx, r.tts.path, r.tts.args)

	cmd.Stdin = strings.NewReader(text)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s: context done: %w", op, ctx.Err())
		}
		return fmt.Errorf("%s: cmd.Run: %w", op, err)
	}

	*buf = outBuf.Bytes()

	return nil
}
