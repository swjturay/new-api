package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchCodexModelsManifestPreservesCodexEnvelopeAndETag(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "0.149.0", r.URL.Query().Get("client_version"))
		require.Equal(t, "Bearer cpa-key", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		require.Equal(t, "W/\"manifest-1\"", r.Header.Get("If-None-Match"))
		w.Header().Set("ETag", "\"manifest-2\"")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"slug":"gpt-5.6-sol","service_tiers":[{"id":"priority","name":"Fast"}],"additional_speed_tiers":["fast"]}]}`))
	}))
	defer server.Close()

	manifest, err := FetchCodexModelsManifest(
		t.Context(),
		server.Client(),
		server.URL,
		"/v1/models",
		"cpa-key",
		"",
		"0.149.0",
		"W/\"manifest-1\"",
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, manifest.StatusCode)
	require.Equal(t, "\"manifest-2\"", manifest.ETag)
	require.False(t, manifest.NotModified)
	require.JSONEq(t, `{"models":[{"slug":"gpt-5.6-sol","service_tiers":[{"id":"priority","name":"Fast"}],"additional_speed_tiers":["fast"]}]}`, string(manifest.Body))
}

func TestFetchCodexModelsManifestReportsNotModified(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "0.149.0", r.URL.Query().Get("client_version"))
		require.Equal(t, "\"manifest-1\"", r.Header.Get("If-None-Match"))
		w.Header().Set("ETag", "\"manifest-1\"")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	manifest, err := FetchCodexModelsManifest(
		t.Context(), server.Client(), server.URL, "/v1/models", "cpa-key", "", "0.149.0", `"manifest-1"`,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotModified, manifest.StatusCode)
	require.True(t, manifest.NotModified)
	require.Equal(t, `"manifest-1"`, manifest.ETag)
	require.Empty(t, manifest.Body)
}

func TestFilterCodexModelsManifestKeepsOnlyAllowedModels(t *testing.T) {
	t.Parallel()

	input := []byte(`{"models":[
        {"slug":"gpt-5.6-sol","service_tiers":[{"id":"priority"}]},
        {"slug":"gpt-5.6-terra","service_tiers":[{"id":"priority"}]}
    ]}`)

	filtered, err := FilterCodexModelsManifest(input, []string{"gpt-5.6-sol"})
	require.NoError(t, err)
	require.JSONEq(t, `{"models":[{"slug":"gpt-5.6-sol","service_tiers":[{"id":"priority"}]}]}`, string(filtered))
}

func TestFilterCodexModelsManifestRejectsInvalidEnvelope(t *testing.T) {
	t.Parallel()

	_, err := FilterCodexModelsManifest([]byte(`{"object":"list","data":[]}`), []string{"gpt-5.6-sol"})
	require.Error(t, err)
}
