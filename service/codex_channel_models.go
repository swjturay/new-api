package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func FetchCodexChannelModels(channel *model.Channel) ([]string, error) {
	if channel == nil || channel.Type != constant.ChannelTypeCodex {
		return nil, fmt.Errorf("channel type is not Codex")
	}
	if channel.ChannelInfo.IsMultiKey {
		return nil, fmt.Errorf("codex channel does not support multi-key model discovery")
	}

	client, err := NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	clientVersion, err := GetLatestCodexClientVersion(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get Codex client version: %w", err)
	}

	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[constant.ChannelTypeCodex]
	}
	return fetchCodexChannelModels(ctx, channel, baseURL, client, clientVersion)
}

// FetchCodexChannelManifest fetches the complete Codex discovery envelope for
// a channel. Direct Codex OAuth channels use ChatGPT's dedicated manifest path;
// OpenAI-compatible channels (including CPA Advanced Custom channels) use the
// upstream /v1/models path with client_version negotiation.
func FetchCodexChannelManifest(
	ctx context.Context,
	channel *model.Channel,
	clientVersion string,
	ifNoneMatch string,
) (*CodexModelsManifest, error) {
	if channel == nil {
		return nil, fmt.Errorf("nil channel")
	}
	baseURL := strings.TrimSpace(channel.GetBaseURL())
	if baseURL == "" {
		return nil, fmt.Errorf("empty channel base URL")
	}
	keys := channel.GetKeys()
	if len(keys) == 0 || strings.TrimSpace(keys[0]) == "" {
		return nil, fmt.Errorf("channel key is required")
	}
	client, err := GetHttpClientWithProxy(channel.GetSetting().Proxy)
	if err != nil {
		return nil, err
	}
	if channel.Type == constant.ChannelTypeCodex {
		oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(keys[0]))
		if err != nil {
			return nil, err
		}
		return FetchCodexModelsManifestCached(ctx, client, baseURL, "/backend-api/codex/models", oauthKey.AccessToken, oauthKey.AccountID, clientVersion, ifNoneMatch)
	}
	return FetchCodexModelsManifestCached(ctx, client, baseURL, "/v1/models", strings.TrimSpace(keys[0]), "", clientVersion, ifNoneMatch)
}

func fetchCodexChannelModels(
	ctx context.Context,
	channel *model.Channel,
	baseURL string,
	client *http.Client,
	clientVersion string,
) ([]string, error) {
	oauthKey, err := parseCodexOAuthKey(strings.TrimSpace(channel.Key))
	if err != nil {
		return nil, err
	}

	statusCode, models, err := FetchCodexModels(ctx, client, baseURL, oauthKey, clientVersion)
	if err != nil {
		return nil, err
	}
	if statusCode == http.StatusUnauthorized {
		if channel.Id <= 0 {
			return nil, fmt.Errorf("codex channel credential expired; save the channel before retrying model fetch")
		}
		refreshedKey, _, refreshErr := RefreshCodexChannelCredential(
			ctx,
			channel.Id,
			CodexCredentialRefreshOptions{ResetCaches: true},
		)
		if refreshErr != nil {
			return nil, fmt.Errorf("failed to refresh Codex channel credential: %w", refreshErr)
		}
		statusCode, models, err = FetchCodexModels(ctx, client, baseURL, &CodexOAuthKey{
			AccessToken: refreshedKey.AccessToken,
			AccountID:   refreshedKey.AccountID,
		}, clientVersion)
		if err != nil {
			return nil, err
		}
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("upstream status: %d", statusCode)
	}
	return models, nil
}
