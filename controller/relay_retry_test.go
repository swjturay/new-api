package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldRetryTraversesCredentialLocal429EvenWhenMarkedSkip(t *testing.T) {
	t.Parallel()

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	err := types.NewOpenAIError(
		errors.New("credential rate limited"),
		types.ErrorCodeBadResponse,
		http.StatusTooManyRequests,
		types.ErrOptionWithSkipRetry(),
	)

	require.True(t, shouldRetry(ctx, err, 1))
}
