package main

import (
	"context"
	"log"
	"os"

	"ai-localbase/internal/auth"
	"ai-localbase/internal/config"
	"ai-localbase/internal/handler"
	"ai-localbase/internal/mcp"
	"ai-localbase/internal/model"
	"ai-localbase/internal/router"
	"ai-localbase/internal/service"
)

func main() {
	serverConfig := config.LoadServerConfig()

	if err := os.MkdirAll(serverConfig.UploadDir, 0o755); err != nil {
		log.Fatalf("failed to create upload directory: %v", err)
	}

	qdrantService := service.NewQdrantService(serverConfig)
	if qdrantService != nil && qdrantService.IsEnabled() {
		if err := qdrantService.Ping(context.Background()); err != nil {
			log.Printf("qdrant ping failed: %v", err)
		} else {
			log.Printf("qdrant connected: %s", serverConfig.QdrantURL)
		}
	}

	stateStore := service.NewAppStateStore(serverConfig.StateFile)
	chatHistoryStore, err := service.NewSQLiteChatHistoryStore(serverConfig.ChatHistoryFile)
	if err != nil {
		log.Fatalf("failed to initialize sqlite chat history store: %v", err)
	}
	defer func() {
		if closeErr := chatHistoryStore.Close(); closeErr != nil {
			log.Printf("failed to close sqlite chat history store: %v", closeErr)
		}
	}()

	userStore, err := service.NewUserStore(serverConfig.AuthDBFile, configEncKey(serverConfig))
	if err != nil {
		log.Fatalf("failed to initialize auth store: %v", err)
	}
	defer func() {
		if closeErr := userStore.Close(); closeErr != nil {
			log.Printf("failed to close auth store: %v", closeErr)
		}
	}()
	if serverConfig.JWTSecret == "dev-secret-change-in-prod" {
		log.Printf("WARNING: JWT_SECRET is using the default dev value; set JWT_SECRET in production")
	}

	appService := service.NewAppService(qdrantService, stateStore, chatHistoryStore, serverConfig, userStore)
	llmService := service.NewLLMService()
	modelProbeService := service.NewModelProbeService()
	mcpRegistry := mcp.DefaultRegistry(appService)
	toolPlanner := mcp.NewToolUsePlanner(mcpRegistry)
	appHandler := handler.NewAppHandler(serverConfig, appService, llmService, toolPlanner, userStore, modelProbeService)
	mcpServer := mcp.NewServer(mcpRegistry, appService, serverConfig)
	r := router.NewRouter(appHandler, serverConfig, mcpServer, frontendFS())

	log.Printf("backend server listening on :%s", serverConfig.Port)
	if err := r.Run(":" + serverConfig.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

// configEncKey resolves the master key used to encrypt user API keys at rest.
// Reads CONFIG_ENCRYPTION_KEY (base64 32-byte or raw passphrase). Falls back
// to a SHA-256 derivation of JWT_SECRET with a loud warning — fine for dev,
// not recommended in production because rotating JWT_SECRET would break decryption.
func configEncKey(cfg model.ServerConfig) []byte {
	key, derived := auth.DeriveConfigEncryptionKey(cfg.ConfigEncryptionKey, cfg.JWTSecret)
	if derived {
		log.Printf("WARNING: CONFIG_ENCRYPTION_KEY not set; derived from JWT_SECRET. Set CONFIG_ENCRYPTION_KEY (base64 32-byte) in production to decouple key rotation.")
	}
	return key
}
