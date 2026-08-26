package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOaiResponsesStreamHandlerEmitsTerminalFailureOnEarlyEOF(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		ChannelMeta:        &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.6"},
		IsStream:           true,
		DisablePing:        true,
		RelayFormat:        types.RelayFormatOpenAIResponses,
		ShouldIncludeUsage: true,
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_early","model":"gpt-5.6"}}`,
			`data: {"type":"response.output_text.delta","delta":"partial"}`,
			"",
		}, "\n"))),
		Header: http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	_, err := OaiResponsesStreamHandler(c, info, resp)
	require.Error(t, err)
	assert.True(t, types.IsSkipRetryError(err))
	assert.True(t, common.GetContextKeyBool(c, constant.ContextKeyStreamTerminalSent))
	assert.Contains(t, recorder.Body.String(), "event: response.failed")
	assert.Contains(t, recorder.Body.String(), `"code":"stream_incomplete"`)
	assert.Contains(t, recorder.Body.String(), "data: [DONE]")
}
