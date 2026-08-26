package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/sync/singleflight"
)

const (
	codexLatestReleaseURL      = "https://api.github.com/repos/openai/codex/releases/latest"
	codexClientVersionCacheTTL = time.Hour
)

type codexClientVersionCache struct {
	sync.Mutex
	version   string
	expiresAt time.Time
}

// CodexModelsManifest is the response envelope used by Codex model discovery.
// The body is intentionally kept opaque so provider-specific capability fields
// such as service_tiers survive the New API hop unchanged.
type CodexModelsManifest struct {
	StatusCode  int
	Body        []byte
	ETag        string
	NotModified bool
}

func CodexModelsManifestETag(body []byte) string {
	digest := sha256.Sum256(body)
	return `"` + hex.EncodeToString(digest[:]) + `"`
}

// FilterCodexModelsManifest limits a complete Codex manifest to the models
// visible through the authenticated New API group. The Codex envelope and all
// per-model capability metadata are preserved.
func FilterCodexModelsManifest(body []byte, allowedModels []string) ([]byte, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode Codex models manifest: %w", err)
	}
	if envelope == nil {
		return nil, fmt.Errorf("decode Codex models manifest: expected object")
	}
	rawModels, ok := envelope["models"]
	if !ok {
		return nil, fmt.Errorf("decode Codex models manifest: missing models")
	}
	var models []json.RawMessage
	if err := json.Unmarshal(rawModels, &models); err != nil {
		return nil, fmt.Errorf("decode Codex models manifest models: %w", err)
	}
	allowed := make(map[string]struct{}, len(allowedModels))
	for _, modelName := range allowedModels {
		if modelName = strings.TrimSpace(modelName); modelName != "" {
			allowed[modelName] = struct{}{}
		}
	}
	filtered := make([]json.RawMessage, 0, len(models))
	for _, rawModel := range models {
		var descriptor struct {
			Slug string `json:"slug"`
		}
		if err := json.Unmarshal(rawModel, &descriptor); err != nil {
			return nil, fmt.Errorf("decode Codex model descriptor: %w", err)
		}
		if _, ok := allowed[strings.TrimSpace(descriptor.Slug)]; ok {
			filtered = append(filtered, rawModel)
		}
	}
	encodedModels, err := json.Marshal(filtered)
	if err != nil {
		return nil, fmt.Errorf("encode filtered Codex models: %w", err)
	}
	envelope["models"] = encodedModels
	filteredBody, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode filtered Codex manifest: %w", err)
	}
	return filteredBody, nil
}

var latestCodexClientVersion codexClientVersionCache

const codexManifestCacheTTL = 30 * time.Second

type codexManifestCacheEntry struct {
	manifest  *CodexModelsManifest
	expiresAt time.Time
}

var codexManifestCache = struct {
	sync.Mutex
	entries map[string]codexManifestCacheEntry
	group   singleflight.Group
}{entries: make(map[string]codexManifestCacheEntry)}

func codexManifestCacheKey(baseURL, endpointPath, bearerToken, accountID, clientVersion string) string {
	tokenDigest := sha256.Sum256([]byte(strings.TrimSpace(bearerToken)))
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + "\x00" + endpointPath + "\x00" + hex.EncodeToString(tokenDigest[:]) + "\x00" + strings.TrimSpace(accountID) + "\x00" + clientVersion
}

// FetchCodexModelsManifestCached provides a short-lived, request-coalesced
// manifest cache. If-None-Match is evaluated against the cached final body.
func FetchCodexModelsManifestCached(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	endpointPath string,
	bearerToken string,
	accountID string,
	clientVersion string,
	ifNoneMatch string,
) (*CodexModelsManifest, error) {
	key := codexManifestCacheKey(baseURL, endpointPath, bearerToken, accountID, clientVersion)
	now := time.Now()
	codexManifestCache.Lock()
	entry, ok := codexManifestCache.entries[key]
	codexManifestCache.Unlock()
	if ok && now.Before(entry.expiresAt) {
		return cloneCodexManifestForCondition(entry.manifest, ifNoneMatch), nil
	}
	value, err, _ := codexManifestCache.group.Do(key, func() (any, error) {
		codexManifestCache.Lock()
		entry, ok := codexManifestCache.entries[key]
		codexManifestCache.Unlock()
		if ok && time.Now().Before(entry.expiresAt) {
			return entry.manifest, nil
		}
		manifest, err := FetchCodexModelsManifest(ctx, client, baseURL, endpointPath, bearerToken, accountID, clientVersion, "")
		if err != nil {
			return nil, err
		}
		if manifest.StatusCode >= http.StatusOK && manifest.StatusCode < http.StatusMultipleChoices {
			codexManifestCache.Lock()
			codexManifestCache.entries[key] = codexManifestCacheEntry{manifest: cloneCodexManifest(manifest), expiresAt: time.Now().Add(codexManifestCacheTTL)}
			codexManifestCache.Unlock()
		}
		return manifest, nil
	})
	if err != nil {
		return nil, err
	}
	return cloneCodexManifestForCondition(value.(*CodexModelsManifest), ifNoneMatch), nil
}

func cloneCodexManifest(manifest *CodexModelsManifest) *CodexModelsManifest {
	if manifest == nil {
		return nil
	}
	clone := *manifest
	clone.Body = append([]byte(nil), manifest.Body...)
	return &clone
}

func cloneCodexManifestForCondition(manifest *CodexModelsManifest, ifNoneMatch string) *CodexModelsManifest {
	clone := cloneCodexManifest(manifest)
	if clone == nil {
		return nil
	}
	if strings.TrimSpace(ifNoneMatch) != "" && strings.TrimSpace(ifNoneMatch) == strings.TrimSpace(clone.ETag) {
		clone.StatusCode = http.StatusNotModified
		clone.NotModified = true
		clone.Body = nil
	}
	return clone
}

func GetLatestCodexClientVersion(ctx context.Context, client *http.Client) (string, error) {
	return latestCodexClientVersion.get(ctx, client, codexLatestReleaseURL, time.Now())
}

func (cache *codexClientVersionCache) get(ctx context.Context, client *http.Client, releaseURL string, now time.Time) (string, error) {
	cache.Lock()
	defer cache.Unlock()

	if cache.version != "" && now.Before(cache.expiresAt) {
		return cache.version, nil
	}

	version, err := fetchLatestCodexClientVersion(ctx, client, releaseURL)
	if err != nil {
		if cache.version != "" {
			cache.expiresAt = now.Add(codexClientVersionCacheTTL)
			return cache.version, nil
		}
		return "", err
	}

	cache.version = version
	cache.expiresAt = now.Add(codexClientVersionCacheTTL)
	return version, nil
}

func fetchLatestCodexClientVersion(ctx context.Context, client *http.Client, releaseURL string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("nil http client")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "new-api")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("codex release lookup failed: status=%d", resp.StatusCode)
	}

	var release struct {
		Name       string `json:"name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := common.DecodeJson(resp.Body, &release); err != nil {
		return "", err
	}
	if release.Draft || release.Prerelease {
		return "", fmt.Errorf("latest codex release is not stable")
	}
	version := strings.TrimSpace(release.Name)
	if version == "" {
		return "", fmt.Errorf("latest codex release has no version name")
	}
	return version, nil
}

// FetchCodexModelsManifest fetches a Codex-compatible models manifest from an
// upstream provider. endpointPath is explicit because CPA uses /v1/models,
// while a direct Codex OAuth upstream uses /backend-api/codex/models.
func FetchCodexModelsManifest(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	endpointPath string,
	bearerToken string,
	accountID string,
	clientVersion string,
	ifNoneMatch string,
) (*CodexModelsManifest, error) {
	if client == nil {
		return nil, fmt.Errorf("nil http client")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	endpointPath = "/" + strings.TrimLeft(strings.TrimSpace(endpointPath), "/")
	bearerToken = strings.TrimSpace(bearerToken)
	accountID = strings.TrimSpace(accountID)
	clientVersion = strings.TrimSpace(clientVersion)
	if baseURL == "" {
		return nil, fmt.Errorf("empty baseURL")
	}
	if bearerToken == "" {
		return nil, fmt.Errorf("codex models: bearer token is required")
	}
	if clientVersion == "" {
		return nil, fmt.Errorf("codex models: client_version is required")
	}

	modelsURL, err := url.Parse(baseURL + endpointPath)
	if err != nil {
		return nil, err
	}
	query := modelsURL.Query()
	query.Set("client_version", clientVersion)
	modelsURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	if accountID != "" {
		req.Header.Set("ChatGPT-Account-Id", accountID)
	}
	req.Header.Set("User-Agent", "codex-cli/"+clientVersion)
	req.Header.Set("Accept", "application/json")
	if ifNoneMatch = strings.TrimSpace(ifNoneMatch); ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &CodexModelsManifest{
		StatusCode:  resp.StatusCode,
		Body:        body,
		ETag:        resp.Header.Get("ETag"),
		NotModified: resp.StatusCode == http.StatusNotModified,
	}, nil
}

func FetchCodexModels(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	oauthKey *CodexOAuthKey,
	clientVersion string,
) (statusCode int, models []string, err error) {
	if client == nil {
		return 0, nil, fmt.Errorf("nil http client")
	}
	if oauthKey == nil {
		return 0, nil, fmt.Errorf("nil oauth key")
	}

	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)
	clientVersion = strings.TrimSpace(clientVersion)
	if baseURL == "" {
		return 0, nil, fmt.Errorf("empty baseURL")
	}
	if accessToken == "" {
		return 0, nil, fmt.Errorf("codex channel: access_token is required")
	}
	if accountID == "" {
		return 0, nil, fmt.Errorf("codex channel: account_id is required")
	}
	if clientVersion == "" {
		return 0, nil, fmt.Errorf("codex channel: client_version is required")
	}

	manifest, err := FetchCodexModelsManifest(ctx, client, baseURL, "/backend-api/codex/models", accessToken, accountID, clientVersion, "")
	if err != nil {
		return 0, nil, err
	}
	if manifest.StatusCode < http.StatusOK || manifest.StatusCode >= http.StatusMultipleChoices {
		return manifest.StatusCode, nil, nil
	}

	var result struct {
		Models []struct {
			Slug string `json:"slug"`
		} `json:"models"`
	}
	if err := common.Unmarshal(manifest.Body, &result); err != nil {
		return manifest.StatusCode, nil, err
	}

	seen := make(map[string]struct{}, len(result.Models))
	models = make([]string, 0, len(result.Models))
	for _, item := range result.Models {
		slug := strings.TrimSpace(item.Slug)
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		models = append(models, slug)
	}
	return manifest.StatusCode, models, nil
}
