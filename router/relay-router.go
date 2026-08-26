package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

func SetRelayRouter(router *gin.Engine) {
	router.Use(middleware.CORS())
	router.Use(middleware.DecompressRequestMiddleware())
	router.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	router.Use(middleware.StatsMiddleware())
	// https://platform.openai.com/docs/api-reference/introduction
	modelsRouter := router.Group("/v1/models")
	modelsRouter.Use(middleware.RouteTag("relay"))
	modelsRouter.Use(middleware.TokenAuth())
	{
		modelsRouter.GET("", func(c *gin.Context) {
			if c.Query("client_version") != "" {
				controller.ListCodexModels(c)
				return
			}
			switch {
			case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
				controller.ListModels(c, constant.ChannelTypeAnthropic)
			case c.GetHeader("x-goog-api-key") != "" || c.Query("key") != "": // 单独的适配
				controller.ListModels(c, constant.ChannelTypeGemini)
			default:
				controller.ListModels(c, constant.ChannelTypeOpenAI)
			}
		})

		modelsRouter.GET("/:model", func(c *gin.Context) {
			switch {
			case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
				controller.RetrieveModel(c, constant.ChannelTypeAnthropic)
			default:
				controller.RetrieveModel(c, constant.ChannelTypeOpenAI)
			}
		})
	}

	geminiRouter := router.Group("/v1beta/models")
	geminiRouter.Use(middleware.RouteTag("relay"))
	geminiRouter.Use(middleware.TokenAuth())
	{
		geminiRouter.GET("", func(c *gin.Context) {
			controller.ListModels(c, constant.ChannelTypeGemini)
		})
	}

	geminiCompatibleRouter := router.Group("/v1beta/openai/models")
	geminiCompatibleRouter.Use(middleware.RouteTag("relay"))
	geminiCompatibleRouter.Use(middleware.TokenAuth())
	{
		geminiCompatibleRouter.GET("", func(c *gin.Context) {
			controller.ListModels(c, constant.ChannelTypeOpenAI)
		})
	}

	playgroundRouter := router.Group("/pg")
	playgroundRouter.Use(middleware.RouteTag("relay"))
	playgroundRouter.Use(middleware.SystemPerformanceCheck())
	playgroundRouter.Use(middleware.UserAuth(), middleware.Distribute())
	{
		playgroundRouter.POST("/chat/completions", controller.Playground)
	}
	relayV1Router := router.Group("/v1")
	relayV1Router.Use(middleware.RouteTag("relay"))
	relayV1Router.Use(middleware.SystemPerformanceCheck())
	relayV1Router.Use(middleware.TokenAuth())
	relayV1Router.Use(middleware.ModelRequestRateLimit())
	{
		// WebSocket 路由（统一到 Relay）
		wsRouter := relayV1Router.Group("")
		wsRouter.Use(middleware.Distribute())
		wsRouter.GET("/realtime", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIRealtime)
		})
		// Responses WebSocket is intentionally not implemented in this
		// migration. Return a deterministic SSE-only response instead of letting
		// the request fall through to the dashboard SPA.
		relayV1Router.GET("/responses", responsesSSEOnly)
	}
	{
		//http router
		httpRouter := relayV1Router.Group("")
		httpRouter.Use(middleware.Distribute())

		// claude related routes
		httpRouter.POST("/messages", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatClaude)
		})

		// chat related routes
		httpRouter.POST("/completions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})
		httpRouter.POST("/chat/completions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})

		// response related routes
		httpRouter.POST("/responses", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIResponses)
		})
		httpRouter.POST("/responses/compact", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIResponsesCompaction)
		})

		// alpha search related routes (Codex standalone web search)
		httpRouter.POST("/alpha/search", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAlphaSearch)
		})

		// image related routes
		httpRouter.POST("/edits", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIImage)
		})
		httpRouter.POST("/images/generations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIImage)
		})
		httpRouter.POST("/images/edits", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIImage)
		})

		// embedding related routes
		httpRouter.POST("/embeddings", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatEmbedding)
		})

		// audio related routes
		httpRouter.POST("/audio/transcriptions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})
		httpRouter.POST("/audio/translations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})
		httpRouter.POST("/audio/speech", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})

		// rerank related routes
		httpRouter.POST("/rerank", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatRerank)
		})

		// gemini relay routes
		httpRouter.POST("/engines/:model/embeddings", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatGemini)
		})
		httpRouter.POST("/models/*path", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatGemini)
		})

		// other relay routes
		httpRouter.POST("/moderations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})

		// not implemented
		httpRouter.POST("/images/variations", controller.RelayNotImplemented)
		httpRouter.GET("/files", controller.RelayNotImplemented)
		httpRouter.POST("/files", controller.RelayNotImplemented)
		httpRouter.DELETE("/files/:id", controller.RelayNotImplemented)
		httpRouter.GET("/files/:id", controller.RelayNotImplemented)
		httpRouter.GET("/files/:id/content", controller.RelayNotImplemented)
		httpRouter.POST("/fine-tunes", controller.RelayNotImplemented)
		httpRouter.GET("/fine-tunes", controller.RelayNotImplemented)
		httpRouter.GET("/fine-tunes/:id", controller.RelayNotImplemented)
		httpRouter.POST("/fine-tunes/:id/cancel", controller.RelayNotImplemented)
		httpRouter.GET("/fine-tunes/:id/events", controller.RelayNotImplemented)
		httpRouter.DELETE("/models/:model", controller.RelayNotImplemented)
	}

	relayMjRouter := router.Group("/mj")
	relayMjRouter.Use(middleware.RouteTag("relay"))
	relayMjRouter.Use(middleware.SystemPerformanceCheck())
	registerMjRouterGroup(relayMjRouter)

	relayMjModeRouter := router.Group("/:mode/mj")
	relayMjModeRouter.Use(middleware.RouteTag("relay"))
	relayMjModeRouter.Use(middleware.SystemPerformanceCheck())
	registerMjRouterGroup(relayMjModeRouter)
	//relayMjRouter.Use()

	relaySunoRouter := router.Group("/suno")
	relaySunoRouter.Use(middleware.RouteTag("relay"))
	relaySunoRouter.Use(middleware.SystemPerformanceCheck())
	relaySunoRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		relaySunoRouter.POST("/submit/:action", controller.RelayTask)
		relaySunoRouter.POST("/fetch", controller.RelayTaskFetch)
		relaySunoRouter.GET("/fetch/:id", controller.RelayTaskFetch)
	}

	relayGeminiRouter := router.Group("/v1beta")
	relayGeminiRouter.Use(middleware.RouteTag("relay"))
	relayGeminiRouter.Use(middleware.SystemPerformanceCheck())
	relayGeminiRouter.Use(middleware.TokenAuth())
	relayGeminiRouter.Use(middleware.ModelRequestRateLimit())
	relayGeminiRouter.Use(middleware.Distribute())
	{
		// Gemini API 路径格式: /v1beta/models/{model_name}:{action}
		relayGeminiRouter.POST("/models/*path", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatGemini)
		})
	}

	// Sub2API's recommended Codex configuration uses the host root as its base
	// URL, so clients append /responses (and sometimes /models) without /v1.
	// Keep these aliases inside the normal relay middleware chain and normalize
	// the request path before distribution and adapter selection.
	registerRelayPathAlias(router, "/responses", "/v1/responses", types.RelayFormatOpenAIResponses)
	registerRelayPathAlias(router, "/responses/compact", "/v1/responses/compact", types.RelayFormatOpenAIResponsesCompaction)
	registerRelayPathAlias(router, "/messages", "/v1/messages", types.RelayFormatClaude)
	registerRelayPathAlias(router, "/completions", "/v1/completions", types.RelayFormatOpenAI)
	registerRelayPathAlias(router, "/chat/completions", "/v1/chat/completions", types.RelayFormatOpenAI)
	registerRelayPathAlias(router, "/alpha/search", "/v1/alpha/search", types.RelayFormatOpenAIAlphaSearch)

	rootResponses := router.Group("")
	rootResponses.Use(relayPathAlias("/v1/responses"))
	rootResponses.Use(middleware.RouteTag("relay"))
	rootResponses.Use(middleware.SystemPerformanceCheck())
	rootResponses.Use(middleware.TokenAuth())
	rootResponses.GET("/responses", responsesSSEOnly)

	rootModels := router.Group("")
	rootModels.Use(relayPathAlias("/v1/models"))
	rootModels.Use(middleware.RouteTag("relay"))
	rootModels.Use(middleware.TokenAuth())
	rootModels.GET("/models", func(c *gin.Context) {
		if c.Query("client_version") != "" {
			controller.ListCodexModels(c)
			return
		}
		switch {
		case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
			controller.ListModels(c, constant.ChannelTypeAnthropic)
		case c.GetHeader("x-goog-api-key") != "" || c.Query("key") != "":
			controller.ListModels(c, constant.ChannelTypeGemini)
		default:
			controller.ListModels(c, constant.ChannelTypeOpenAI)
		}
	})

	// Codex-compatible alias used by clients configured with a ChatGPT-style
	// backend base URL. The handler still requires client_version to select the
	// manifest response; without it, normal OpenAI model-list semantics apply.
	codexModelsAlias := router.Group("/backend-api/codex")
	codexModelsAlias.Use(middleware.RouteTag("relay"))
	codexModelsAlias.Use(middleware.SystemPerformanceCheck())
	codexModelsAlias.Use(middleware.TokenAuth())
	codexModelsAlias.GET("/models", controller.ListCodexModels)
}

func registerRelayPathAlias(router *gin.Engine, aliasPath, canonicalPath string, relayFormat types.RelayFormat) {
	alias := router.Group("")
	alias.Use(relayPathAlias(canonicalPath))
	alias.Use(middleware.RouteTag("relay"))
	alias.Use(middleware.SystemPerformanceCheck())
	alias.Use(middleware.TokenAuth())
	alias.Use(middleware.ModelRequestRateLimit())
	alias.Use(middleware.Distribute())
	alias.POST(aliasPath, func(c *gin.Context) {
		controller.Relay(c, relayFormat)
	})
}

func relayPathAlias(canonicalPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request != nil && c.Request.URL != nil {
			c.Request.URL.Path = canonicalPath
			if c.Request.URL.RawQuery != "" {
				c.Request.RequestURI = canonicalPath + "?" + c.Request.URL.RawQuery
			} else {
				c.Request.RequestURI = canonicalPath
			}
		}
		c.Next()
	}
}

func responsesSSEOnly(c *gin.Context) {
	c.Header("X-New-API-Transport", "sse")
	c.Header("X-New-API-SSE-Endpoint", "/v1/responses")
	c.JSON(http.StatusUpgradeRequired, gin.H{
		"error": types.OpenAIError{
			Message: "Responses WebSocket is not enabled; use HTTP streaming (SSE) at /v1/responses",
			Type:    "invalid_request_error",
			Param:   "",
			Code:    "responses_websocket_disabled",
		},
	})
}

func registerMjRouterGroup(relayMjRouter *gin.RouterGroup) {
	relayMjRouter.GET("/image/:id", relay.RelayMidjourneyImage)
	relayMjRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		relayMjRouter.POST("/submit/action", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/shorten", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/modal", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/imagine", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/change", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/simple-change", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/describe", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/blend", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/edits", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/video", controller.RelayMidjourney)
		//relayMjRouter.POST("/notify", controller.RelayMidjourney)
		relayMjRouter.GET("/task/:id/fetch", controller.RelayMidjourney)
		relayMjRouter.GET("/task/:id/image-seed", controller.RelayMidjourney)
		relayMjRouter.POST("/task/list-by-condition", controller.RelayMidjourney)
		relayMjRouter.POST("/insight-face/swap", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/upload-discord-images", controller.RelayMidjourney)
	}
}
