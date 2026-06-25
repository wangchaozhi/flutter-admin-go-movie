package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"flutter-admin-go/internal/config"
)

var (
	ErrDisabled            = errors.New("ai metadata generation is disabled")
	ErrUnsupportedProvider = errors.New("unsupported ai provider")
)

type VideoMetadataInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Category    string   `json:"category,omitempty"`
	Actors      []string `json:"actors,omitempty"`
	Directors   []string `json:"directors,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	Region      string   `json:"region,omitempty"`
	ReleaseYear int      `json:"release_year,omitempty"`
	Language    string   `json:"language,omitempty"`
	Duration    int      `json:"duration,omitempty"`
	Width       int      `json:"width,omitempty"`
	Height      int      `json:"height,omitempty"`
	IsVIP       bool     `json:"is_vip"`
	IsFree      bool     `json:"is_free"`
}

type VideoMetadata struct {
	Synopsis   string   `json:"synopsis"`
	Highlights []string `json:"highlights"`
	Tags       []string `json:"tags"`
}

type Provider interface {
	Name() string
	Model() string
	GenerateVideoMetadata(ctx context.Context, input VideoMetadataInput) (VideoMetadata, error)
}

func NewProvider(cfg config.AIConfig) (Provider, error) {
	if !cfg.Enabled {
		return nil, ErrDisabled
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	switch provider {
	case "deepseek", "openai-compatible", "openai_compatible":
		return newOpenAICompatibleProvider(cfg)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, cfg.Provider)
	}
}

type openAICompatibleProvider struct {
	name    string
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func newOpenAICompatibleProvider(cfg config.AIConfig) (*openAICompatibleProvider, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("ai api key is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "deepseek-v4-flash"
	}
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 45
	}
	name := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if name == "" {
		name = "openai-compatible"
	}
	return &openAICompatibleProvider{
		name:    name,
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *openAICompatibleProvider) Name() string { return p.name }

func (p *openAICompatibleProvider) Model() string { return p.model }

func (p *openAICompatibleProvider) GenerateVideoMetadata(ctx context.Context, input VideoMetadataInput) (VideoMetadata, error) {
	payload := chatCompletionRequest{
		Model: p.model,
		Messages: []chatMessage{
			{Role: "system", Content: videoMetadataSystemPrompt},
			{Role: "user", Content: buildVideoMetadataPrompt(input)},
		},
		ResponseFormat: map[string]string{"type": "json_object"},
		Temperature:    0.4,
		MaxTokens:      900,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return VideoMetadata{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return VideoMetadata{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return VideoMetadata{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return VideoMetadata{}, fmt.Errorf("ai provider http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(raw, &completion); err != nil {
		return VideoMetadata{}, fmt.Errorf("parse ai response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return VideoMetadata{}, errors.New("ai response has no choices")
	}
	var metadata VideoMetadata
	if err := json.Unmarshal([]byte(stripJSONFence(completion.Choices[0].Message.Content)), &metadata); err != nil {
		return VideoMetadata{}, fmt.Errorf("parse ai metadata json: %w", err)
	}
	metadata = normalizeVideoMetadata(metadata)
	if metadata.Synopsis == "" {
		return VideoMetadata{}, errors.New("ai metadata synopsis is empty")
	}
	return metadata, nil
}

type chatCompletionRequest struct {
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
	Temperature    float64           `json:"temperature,omitempty"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

const videoMetadataSystemPrompt = `你是视频网站的内容运营编辑。请为站内视频生成面向观众的信息模块。
要求：
- 只根据用户提供的标题、已有简介、分类、演职员、地区、年份、类型、语言、时长、分辨率等元数据创作。
- 演员、导演、地区、年份、语言为空时不要编造；已有时可以自然融入简介或看点。
- 风格接近主流视频网站：清楚、克制、有吸引力，避免营销腔和夸张承诺。
- 输出必须是 JSON 对象，字段只有 synopsis、highlights、tags。
- synopsis 为 80 到 160 个中文字符。
- highlights 为 3 到 5 条，每条 8 到 24 个中文字符。
- tags 为 4 到 8 个短标签，每个 2 到 8 个中文字符。`

func buildVideoMetadataPrompt(input VideoMetadataInput) string {
	raw, _ := json.Marshal(input)
	return "请为这个视频补全播放页信息，返回严格 JSON：\n" + string(raw)
}

func stripJSONFence(value string) string {
	text := strings.TrimSpace(value)
	if !strings.HasPrefix(text, "```") {
		return text
	}
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return strings.TrimSpace(text)
}

func normalizeVideoMetadata(metadata VideoMetadata) VideoMetadata {
	metadata.Synopsis = trimRunes(strings.TrimSpace(metadata.Synopsis), 220)
	metadata.Highlights = normalizeStringList(metadata.Highlights, 5, 36)
	metadata.Tags = normalizeStringList(metadata.Tags, 8, 12)
	return metadata
}

func normalizeStringList(values []string, maxItems, maxRunes int) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		item := trimRunes(strings.TrimSpace(value), maxRunes)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
		if len(result) >= maxItems {
			break
		}
	}
	return result
}

func trimRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}
