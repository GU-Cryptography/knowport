package router

import (
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-localbase/internal/auth"
	"ai-localbase/internal/handler"
	"ai-localbase/internal/mcp"
	"ai-localbase/internal/model"
	"ai-localbase/internal/util"

	"github.com/gin-gonic/gin"
)

func NewRouter(appHandler *handler.AppHandler, serverConfig model.ServerConfig, mcpServer *mcp.Server, frontendFS fs.FS) *gin.Engine {
	r := gin.New()
	r.Use(requestIDMiddleware(), accessLogMiddleware(), gin.Recovery(), corsMiddleware())

	r.GET("/health", appHandler.Health)
	r.POST("/upload", auth.RequireUser(serverConfig.JWTSecret), appHandler.Upload)

	api := r.Group("/api")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", appHandler.Register)
			authGroup.POST("/login", appHandler.Login)
			authGroup.POST("/refresh", appHandler.Refresh)
			authGroup.POST("/logout", auth.RequireUser(serverConfig.JWTSecret), appHandler.Logout)
			authGroup.GET("/me", auth.RequireUser(serverConfig.JWTSecret), appHandler.Me)
		}

		api.POST("/config/mcp/reset-token", auth.RequireUser(serverConfig.JWTSecret), appHandler.ResetMCPToken)

		// All conversation / KB / upload routes require an authenticated user (F1 Phase 3).
		protected := api.Group("", auth.RequireUser(serverConfig.JWTSecret))
		{
			protected.GET("/config", appHandler.GetConfig)
			protected.PUT("/config", appHandler.UpdateConfig)
			protected.POST("/config/models/probe", appHandler.ProbeModels)
			protected.GET("/conversations", appHandler.ListConversations)
			protected.GET("/conversations/:id", appHandler.GetConversation)
			protected.PUT("/conversations/:id", appHandler.SaveConversation)
			protected.DELETE("/conversations/:id", appHandler.DeleteConversation)
			protected.GET("/knowledge-bases", appHandler.ListKnowledgeBases)
			protected.POST("/knowledge-bases", appHandler.CreateKnowledgeBase)
			protected.DELETE("/knowledge-bases/:id", appHandler.DeleteKnowledgeBase)
			protected.POST("/uploads", appHandler.StageUpload)
			protected.GET("/knowledge-bases/:id/documents", appHandler.ListDocuments)
			protected.POST("/knowledge-bases/:id/documents", appHandler.UploadToKnowledgeBase)
			protected.DELETE("/knowledge-bases/:id/documents/:documentId", appHandler.DeleteDocument)
		}
	}

	v1 := r.Group("/v1", auth.RequireUser(serverConfig.JWTSecret))
	{
		v1.POST("/chat/completions", appHandler.ChatCompletions)
		v1.POST("/chat/completions/stream", appHandler.ChatCompletionsStream)
	}

	if serverConfig.EnableMCP && mcpServer != nil {
		basePath := strings.TrimSpace(serverConfig.MCPBasePath)
		if basePath == "" {
			basePath = "/mcp"
		}
		mcpServer.RegisterRoutes(r.Group(basePath))
	}

	r.NoRoute(spaHandler(frontendFS))

	return r
}

func spaHandler(frontendFS fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(frontendFS))
	return func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if path != "" {
			if f, err := frontendFS.Open(path); err == nil {
				f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Expose-Headers", "X-Request-Id")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-Id"))
		if requestID == "" {
			requestID = util.NextRequestID()
		}

		c.Set("requestId", requestID)
		c.Header("X-Request-Id", requestID)
		c.Next()
	}
}

func accessLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		requestID := strings.TrimSpace(c.GetHeader("X-Request-Id"))
		if requestID == "" {
			if value, ok := c.Get("requestId"); ok {
				requestID, _ = value.(string)
			}
		}

		c.Next()

		log.Printf(
			"request_id=%s method=%s path=%s status=%d duration_ms=%d client_ip=%s",
			requestID,
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			time.Since(startedAt).Milliseconds(),
			c.ClientIP(),
		)
	}
}
