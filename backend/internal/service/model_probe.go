package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ProbeRequest carries the inputs for a model-list probe.
// Kind is "chat" or "embedding" — used purely to derive cache keys so the two
// channels do not pollute each other when the URL/key happen to coincide.
type ProbeRequest struct {
	APIURL string
	APIKey string
	Kind   string
}

// ProbeResult is the canonical shape returned to the frontend.
type ProbeResult struct {
	Models   []string `json:"models"`
	Protocol string   `json:"protocol"` // "openai" | "ollama" | "anthropic"
	Cached   bool     `json:"cached"`
}

type probeCacheEntry struct {
	result ProbeResult
	expiry time.Time
}

// ModelProbeService runs ordered protocol probes and caches results for 5min.
// Cache key = sha256(kind|url|apiKey); changing any of them invalidates immediately.
type ModelProbeService struct {
	httpClient *http.Client
	cache      map[string]probeCacheEntry
	cacheMu    sync.Mutex
	cacheTTL   time.Duration
}

func NewModelProbeService() *ModelProbeService {
	return &ModelProbeService{
		httpClient: &http.Client{Timeout: 8 * time.Second},
		cache:      map[string]probeCacheEntry{},
		cacheTTL:   5 * time.Minute,
	}
}

// Anthropic exposes no list-models endpoint, so we fall back to a curated set.
var anthropicWhitelist = []string{
	"claude-opus-4-7",
	"claude-sonnet-4-6",
	"claude-haiku-4-5",
	"claude-3-7-sonnet-latest",
	"claude-3-5-haiku-latest",
}

// Probe runs the probe pipeline. On any protocol match it returns immediately.
func (s *ModelProbeService) Probe(ctx context.Context, req ProbeRequest) (ProbeResult, error) {
	apiURL := strings.TrimSpace(req.APIURL)
	if apiURL == "" {
		return ProbeResult{}, fmt.Errorf("api_url is required")
	}
	apiURL = strings.TrimRight(apiURL, "/")

	key := cacheKey(req.Kind, apiURL, req.APIKey)
	if entry, ok := s.lookup(key); ok {
		entry.Cached = true
		return entry, nil
	}

	// 1. OpenAI-compatible /v1/models. If the URL already ends in /v1, do not double it.
	openaiURL := openaiModelsURL(apiURL)
	if models, err := s.tryOpenAI(ctx, openaiURL, req.APIKey); err == nil && len(models) > 0 {
		result := ProbeResult{Models: models, Protocol: "openai"}
		s.store(key, result)
		return result, nil
	}

	// 2. Ollama /api/tags (no auth header).
	ollamaURL := ollamaTagsURL(apiURL)
	if models, err := s.tryOllama(ctx, ollamaURL); err == nil && len(models) > 0 {
		result := ProbeResult{Models: models, Protocol: "ollama"}
		s.store(key, result)
		return result, nil
	}

	// 3. Anthropic whitelist when host looks like anthropic.com.
	if strings.Contains(strings.ToLower(apiURL), "anthropic.com") {
		result := ProbeResult{Models: append([]string(nil), anthropicWhitelist...), Protocol: "anthropic"}
		s.store(key, result)
		return result, nil
	}

	return ProbeResult{}, fmt.Errorf("no models detected at %s (tried /v1/models and /api/tags)", apiURL)
}

func (s *ModelProbeService) tryOpenAI(ctx context.Context, url, apiKey string) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("openai probe status %d: %s", resp.StatusCode, truncate(body, 200))
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(payload.Data))
	for _, m := range payload.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			out = append(out, id)
		}
	}
	return out, nil
}

func (s *ModelProbeService) tryOllama(ctx context.Context, url string) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("ollama probe status %d: %s", resp.StatusCode, truncate(body, 200))
	}

	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(payload.Models))
	for _, m := range payload.Models {
		if name := strings.TrimSpace(m.Name); name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

func (s *ModelProbeService) lookup(key string) (ProbeResult, bool) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	entry, ok := s.cache[key]
	if !ok {
		return ProbeResult{}, false
	}
	if time.Now().After(entry.expiry) {
		delete(s.cache, key)
		return ProbeResult{}, false
	}
	return entry.result, true
}

func (s *ModelProbeService) store(key string, result ProbeResult) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache[key] = probeCacheEntry{result: result, expiry: time.Now().Add(s.cacheTTL)}
}

func cacheKey(kind, url, apiKey string) string {
	h := sha256.Sum256([]byte(kind + "|" + url + "|" + apiKey))
	return hex.EncodeToString(h[:])
}

// openaiModelsURL maps base → base/v1/models, but preserves existing /v1 or /v1/models suffix.
func openaiModelsURL(base string) string {
	lower := strings.ToLower(strings.TrimRight(strings.TrimSpace(base), "/"))
	switch {
	case strings.HasSuffix(lower, "/v1/models"):
		return strings.TrimRight(strings.TrimSpace(base), "/")
	case strings.HasSuffix(lower, "/v1"):
		return strings.TrimRight(strings.TrimSpace(base), "/") + "/models"
	default:
		return strings.TrimRight(strings.TrimSpace(base), "/") + "/v1/models"
	}
}

// openAIBaseURL normalises an OpenAI-compatible base URL so it always ends in /v1.
// Callers append "/chat/completions" or "/embeddings" themselves. Does not handle
// fully-pathed inputs (e.g. /v1/models) — those belong in openaiModelsURL.
func openAIBaseURL(base string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, "/v1") {
		return trimmed
	}
	return trimmed + "/v1"
}

// ollamaTagsURL maps base → base/api/tags, stripping a trailing /v1 if present
// (Ollama's tag endpoint lives at the root, not under /v1).
func ollamaTagsURL(base string) string {
	lower := strings.ToLower(base)
	if strings.HasSuffix(lower, "/v1") {
		base = base[:len(base)-3]
	}
	if strings.HasSuffix(strings.ToLower(base), "/api/tags") {
		return base
	}
	return base + "/api/tags"
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
