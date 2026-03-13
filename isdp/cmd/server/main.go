package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anthropic/isdp/internal/api"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/a2a"
	"github.com/anthropic/isdp/internal/service/merge"
	"github.com/anthropic/isdp/internal/service/message"
	"github.com/anthropic/isdp/internal/service/project"
	"github.com/anthropic/isdp/internal/service/sandbox"
	"github.com/anthropic/isdp/internal/service/thread"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// 加载配置
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logger, err := initLogger(cfg.Logging.Level, cfg.Logging.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init logger: %v\n", err)
		os.Exit(1)
	}

	// 连接数据库
	db, err := repo.NewDBFromConfig(repo.DBConfig{
		Path: cfg.Database.Path,
	})
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// 初始化数据库表结构
	if err := initDatabase(db); err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// 连接Redis（可选）
	var redisClient repo.RedisClient
	redisClient, err = repo.NewRedis(repo.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		logger.Warn("Redis not available, running without cache", zap.Error(err))
	} else {
		defer redisClient.Close()
	}

	// 初始化WebSocket Hub
	wsHub := ws.NewHub()
	go wsHub.Run()

	// 初始化Repositories
	projectRepo := repo.NewProjectRepository(db)
	threadRepo := repo.NewThreadRepository(db)
	messageRepo := repo.NewMessageRepository(db)
	agentConfigRepo := repo.NewAgentConfigRepository(db)
	invocationRepo := repo.NewAgentInvocationRepository(db)
	artifactRepo := repo.NewArtifactRepository(db)
	sandboxRepo := repo.NewSandboxRepository(db)
	reviewRepo := repo.NewReviewRepository(artifactRepo)

	// 初始化Services
	projectService := project.NewService(projectRepo)
	threadService := thread.NewService(threadRepo)
	messageService := message.NewService(messageRepo, wsHub)
	configService := agent.NewConfigService(agentConfigRepo)
	workflowEngine := agent.NewWorkflowEngine(threadRepo, messageRepo, configService)
	mcpAuthService := a2a.NewMCPAuthService(cfg.MCP.TokenTTL)
	claudeAdapter := agent.NewClaudeAdapter(cfg.Claude.Path)
	_ = agent.NewContextBuilder(threadRepo, messageRepo, artifactRepo) // TODO: wire into orchestrator
	tracker := agent.NewInvocationTracker(invocationRepo)
	orchestrator := agent.NewOrchestrator(
		invocationRepo, threadRepo, messageRepo,
		configService, tracker, workflowEngine, wsHub, claudeAdapter,
	)
	_ = merge.NewGatekeeper(reviewRepo, artifactRepo, threadRepo) // TODO: wire into merge handler

	// 初始化Docker客户端（可选）
	var sandboxService *sandbox.SandboxService
	dockerClient, err := sandbox.NewDockerClient()
	if err != nil {
		logger.Warn("Docker not available, sandbox features disabled", zap.Error(err))
	} else {
		sandboxService = sandbox.NewSandboxService(dockerClient, sandboxRepo)
		_ = sandboxService // TODO: wire into sandbox handler
	}

	// 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 创建路由
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	router.GET("/ready", func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "database"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// API路由组
	v1 := router.Group("/api/v1")

	// 注册Handlers
	projectHandler := api.NewProjectHandler(projectService)
	projectHandler.RegisterRoutes(v1)

	// 先注册 invocationHandler（包含 /threads/:threadId/invocations）
	invocationHandler := api.NewInvocationHandler(orchestrator, mcpAuthService)
	invocationHandler.RegisterRoutes(v1)

	// 再注册 threadHandler（包含 /threads/:id）
	threadHandler := api.NewThreadHandler(threadService)
	threadHandler.RegisterRoutes(v1)

	messageHandler := api.NewMessageHandler(messageService)
	messageHandler.RegisterRoutes(v1)

	agentHandler := api.NewAgentHandler(configService)
	agentHandler.RegisterRoutes(v1)

	// WebSocket
	wsHandler := ws.NewHandler(wsHub)
	wsHandler.RegisterRoutes(v1)

	// 启动服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// 优雅关闭
	go func() {
		logger.Info("Starting server", zap.Int("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func initLogger(level, format string) (*zap.Logger, error) {
	var cfg zap.Config
	if format == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}

	switch level {
	case "debug":
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	}

	return cfg.Build()
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// initDatabase 初始化数据库表结构
func initDatabase(db *sql.DB) error {
	schema := `
-- 项目表
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    mode TEXT NOT NULL,
    status TEXT DEFAULT 'draft',
    git_repo TEXT,
    config TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 开发会话表
CREATE TABLE IF NOT EXISTS threads (
    id TEXT PRIMARY KEY,
    project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT DEFAULT 'idle',
    current_phase TEXT,
    current_agent TEXT,
    depth INTEGER DEFAULT 0,
    abort_token TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 消息表
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    thread_id TEXT REFERENCES threads(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    agent_id TEXT,
    content TEXT,
    message_type TEXT DEFAULT 'text',
    metadata TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Agent配置表
CREATE TABLE IF NOT EXISTS agent_configs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    description TEXT,
    system_prompt TEXT,
    model_name TEXT DEFAULT 'claude-sonnet-4-6',
    max_tokens INTEGER DEFAULT 4096,
    temperature REAL DEFAULT 0.7,
    routing_config TEXT,
    is_default INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Agent调用记录表
CREATE TABLE IF NOT EXISTS agent_invocations (
    id TEXT PRIMARY KEY,
    thread_id TEXT REFERENCES threads(id) ON DELETE CASCADE,
    agent_config_id TEXT,
    role TEXT NOT NULL,
    status TEXT DEFAULT 'running',
    input TEXT,
    output TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 产物表
CREATE TABLE IF NOT EXISTS artifacts (
    id TEXT PRIMARY KEY,
    thread_id TEXT REFERENCES threads(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    name TEXT,
    path TEXT,
    content TEXT,
    metadata TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 沙箱容器表
CREATE TABLE IF NOT EXISTS sandboxes (
    id TEXT PRIMARY KEY,
    thread_id TEXT REFERENCES threads(id) ON DELETE CASCADE,
    name TEXT,
    image TEXT,
    status TEXT DEFAULT 'created',
    container_id TEXT,
    port INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_threads_project_id ON threads(project_id);
CREATE INDEX IF NOT EXISTS idx_messages_thread_id ON messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_agent_invocations_thread_id ON agent_invocations(thread_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_thread_id ON artifacts(thread_id);
`
	_, err := db.Exec(schema)
	return err
}