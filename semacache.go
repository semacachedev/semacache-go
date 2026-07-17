// Package semacache provides a standalone HTTP client for semacache.io.
// Zero dependencies beyond the Go standard library.
//
// Usage:
//
//	client := semacache.NewClient("sc-your-key", nil)
//
//	resp, err := client.CreateChatCompletion(ctx, semacache.ChatCompletionRequest{
//	    Model:    "gpt-4o",
//	    Messages: []semacache.Message{{Role: "user", Content: "Hello"}},
//	})
//	fmt.Println(resp.Choices[0].Message.Content)
//	fmt.Println(resp.Cache.MatchType) // "EXACT", "SEMANTIC", or ""
package semacache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const DefaultBaseURL = "https://www.semacache.io/api/v1"

// Options configures SemaCache client behavior.
type Options struct {
	BaseURL             string
	UpstreamAPIKey      string
	SimilarityThreshold float64
	CacheTTL            int
	Timeout             time.Duration
}

// Client is a standalone SemaCache HTTP client.
type Client struct {
	baseURL    string
	headers    map[string]string
	httpClient *http.Client
}

// NewClient creates a SemaCache client. opts may be nil for defaults.
func NewClient(apiKey string, opts *Options) *Client {
	if opts == nil {
		opts = &Options{}
	}
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  "application/json",
	}
	if opts.UpstreamAPIKey != "" {
		headers["x-upstream-api-key"] = opts.UpstreamAPIKey
	}
	if opts.SimilarityThreshold > 0 {
		headers["x-similarity-threshold"] = strconv.FormatFloat(opts.SimilarityThreshold, 'f', -1, 64)
	}
	if opts.CacheTTL > 0 {
		headers["x-cache-ttl"] = strconv.Itoa(opts.CacheTTL)
	}

	return &Client{
		baseURL: baseURL,
		headers: headers,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest is the request body for chat completions.
//
// Extras lets you forward arbitrary OpenAI-compatible params (``temperature``,
// ``tools``, ``response_format``, ``reasoning_effort``, …) without the SDK
// needing to know about each one. They're merged into the JSON body verbatim.
// Use ``extra_body`` as a key to reach provider-specific extensions.
type ChatCompletionRequest struct {
	Model               string         `json:"model"`
	Messages            []Message      `json:"messages"`
	Extras              map[string]any `json:"-"`
	SimilarityThreshold float64        `json:"-"`
	CacheTTL            int            `json:"-"`
	NoCache             bool           `json:"-"`
	NoStore             bool           `json:"-"`
}

// Choice represents a completion choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CacheInfo contains SemaCache-specific metadata from response headers.
type CacheInfo struct {
	MatchType  string
	Confidence float64
	LatencyMs  float64
}

// ChatCompletionResponse is the response from chat completions.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
	Cache   CacheInfo
}

// ImageGenerateRequest is the request body for image generation.
//
// Extras forwards arbitrary upstream params (``style``, ``response_format``,
// ``seed``, ``negative_prompt``, ``aspect_ratio``, ``extra_body``, …) verbatim.
type ImageGenerateRequest struct {
	Prompt              string         `json:"prompt"`
	Model               string         `json:"model"`
	N                   int            `json:"n"`
	Size                string         `json:"size"`
	Quality             string         `json:"quality"`
	Extras              map[string]any `json:"-"`
	SimilarityThreshold float64        `json:"-"`
	CacheTTL            int            `json:"-"`
	NoCache             bool           `json:"-"`
	NoStore             bool           `json:"-"`
}

// MediaItem represents a single image or video URL in the response.
type MediaItem struct {
	URL string `json:"url"`
}

// ImageGenerateResponse is the response from image generation.
type ImageGenerateResponse struct {
	Data  []MediaItem `json:"data"`
	Cache CacheInfo
}

// VideoGenerateRequest is the request body for video generation.
//
// Extras forwards arbitrary upstream params (``negative_prompt``, ``seed``,
// ``resolution``, ``enhance_prompt``, ``extra_body``, …) verbatim.
type VideoGenerateRequest struct {
	Prompt              string         `json:"prompt"`
	Model               string         `json:"model"`
	DurationSeconds     int            `json:"duration_seconds"`
	AspectRatio         string         `json:"aspect_ratio"`
	N                   int            `json:"n"`
	Extras              map[string]any `json:"-"`
	SimilarityThreshold float64        `json:"-"`
	CacheTTL            int            `json:"-"`
	NoCache             bool           `json:"-"`
	NoStore             bool           `json:"-"`
}

// VideoGenerateResponse is the response from video generation.
type VideoGenerateResponse struct {
	Data  []MediaItem `json:"data"`
	Cache CacheInfo
}

// marshalWithExtras serializes the typed struct and then merges the extras
// map on top so SDK-known fields and arbitrary passthrough params co-exist
// in a single JSON body.
func marshalWithExtras(base any, extras map[string]any) ([]byte, error) {
	raw, err := json.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("semacache: marshal request: %w", err)
	}
	if len(extras) == 0 {
		return raw, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("semacache: re-decode request: %w", err)
	}
	for k, v := range extras {
		m[k] = v
	}
	return json.Marshal(m)
}

// post sends a POST request and returns the raw body and HTTP response.
func (c *Client) post(ctx context.Context, path string, body []byte, threshold float64, ttl int, noCache, noStore bool) ([]byte, *http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("semacache: create request: %w", err)
	}
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	if threshold > 0 {
		httpReq.Header.Set("x-similarity-threshold", strconv.FormatFloat(threshold, 'f', -1, 64))
	}
	if ttl > 0 {
		httpReq.Header.Set("x-cache-ttl", strconv.Itoa(ttl))
	}
	if noCache {
		httpReq.Header.Set("Cache-Control", "no-cache")
	} else if noStore {
		httpReq.Header.Set("Cache-Control", "no-store")
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("semacache: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("semacache: read response: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("semacache: API error %d: %s", httpResp.StatusCode, string(respBody))
	}
	return respBody, httpResp, nil
}

// parseCacheInfo extracts cache metadata from response headers.
func parseCacheInfo(h http.Header) CacheInfo {
	ci := CacheInfo{MatchType: h.Get("x-semcache-match-type")}
	if conf := h.Get("x-semcache-confidence"); conf != "" {
		ci.Confidence, _ = strconv.ParseFloat(conf, 64)
	}
	if lat := h.Get("x-semcache-latency-ms"); lat != "" {
		ci.LatencyMs, _ = strconv.ParseFloat(lat, 64)
	}
	return ci
}

// CreateChatCompletion sends a chat completion request through SemaCache.
func (c *Client) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	body, err := marshalWithExtras(req, req.Extras)
	if err != nil {
		return nil, err
	}
	respBody, httpResp, err := c.post(ctx, "/chat/completions", body, req.SimilarityThreshold, req.CacheTTL, req.NoCache, req.NoStore)
	if err != nil {
		return nil, err
	}
	var resp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("semacache: decode response: %w", err)
	}
	resp.Cache = parseCacheInfo(httpResp.Header)
	return &resp, nil
}

// GenerateImage sends an image generation request through SemaCache.
func (c *Client) GenerateImage(ctx context.Context, req ImageGenerateRequest) (*ImageGenerateResponse, error) {
	if req.Model == "" {
		req.Model = "gpt-image-1"
	}
	if req.N == 0 {
		req.N = 1
	}
	if req.Size == "" {
		req.Size = "1024x1024"
	}
	if req.Quality == "" {
		req.Quality = "standard"
	}
	body, err := marshalWithExtras(req, req.Extras)
	if err != nil {
		return nil, err
	}
	respBody, httpResp, err := c.post(ctx, "/images/generations", body, req.SimilarityThreshold, req.CacheTTL, req.NoCache, req.NoStore)
	if err != nil {
		return nil, err
	}
	var resp ImageGenerateResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("semacache: decode response: %w", err)
	}
	resp.Cache = parseCacheInfo(httpResp.Header)
	return &resp, nil
}

// GenerateVideo sends a video generation request through SemaCache.
func (c *Client) GenerateVideo(ctx context.Context, req VideoGenerateRequest) (*VideoGenerateResponse, error) {
	if req.Model == "" {
		req.Model = "veo-2.0-generate-001"
	}
	if req.DurationSeconds == 0 {
		req.DurationSeconds = 8
	}
	if req.AspectRatio == "" {
		req.AspectRatio = "16:9"
	}
	if req.N == 0 {
		req.N = 1
	}
	body, err := marshalWithExtras(req, req.Extras)
	if err != nil {
		return nil, err
	}
	respBody, httpResp, err := c.post(ctx, "/videos/generations", body, req.SimilarityThreshold, req.CacheTTL, req.NoCache, req.NoStore)
	if err != nil {
		return nil, err
	}
	var resp VideoGenerateResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("semacache: decode response: %w", err)
	}
	resp.Cache = parseCacheInfo(httpResp.Header)
	return &resp, nil
}
