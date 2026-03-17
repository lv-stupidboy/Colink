package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	"github.com/anthropic/isdp/internal/service/workflow"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	// 设置 Windows 控制台 UTF-8 编码，解决中文乱码问题
	os.Stdout.WriteString("\x1b[?65001h")
	os.Stderr.WriteString("\x1b[?65001h")

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

	// 设置会话日志记录器
	agent.SetSessionLogger(logger)

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
	baseAgentRepo := repo.NewBaseAgentRepository(db)
	invocationRepo := repo.NewAgentInvocationRepository(db)
	artifactRepo := repo.NewArtifactRepository(db)
	sandboxRepo := repo.NewSandboxRepository(db)
	reviewRepo := repo.NewReviewRepository(artifactRepo)
	workflowRepo := repo.NewWorkflowTemplateRepository(db)

	// 初始化Services
	projectService := project.NewService(projectRepo, workflowRepo)
	threadService := thread.NewService(threadRepo, projectRepo, workflowRepo)
	messageService := message.NewService(messageRepo, wsHub)
	configService := agent.NewConfigService(agentConfigRepo)
	baseAgentService := agent.NewBaseAgentService(baseAgentRepo)
	workflowEngine := agent.NewWorkflowEngine(threadRepo, messageRepo, configService)
	workflowService := workflow.NewService(workflowRepo)
	mcpAuthService := a2a.NewMCPAuthService(cfg.MCP.TokenTTL)

	// 初始化默认基础Agent
	if err := baseAgentService.InitDefaultAgents(context.Background()); err != nil {
		logger.Warn("Failed to initialize default base agents", zap.Error(err))
	}

	// 初始化系统工作流模板
	if err := workflowService.InitSystemTemplates(context.Background()); err != nil {
		logger.Warn("Failed to initialize system workflow templates", zap.Error(err))
	}

	// 初始化适配器（使用默认Claude适配器，后续会改为从BaseAgent动态创建）
	claudeAdapter := agent.NewClaudeAdapter(cfg.Claude.Path)
	_ = agent.NewContextBuilder(threadRepo, messageRepo, artifactRepo) // TODO: wire into orchestrator
	tracker := agent.NewInvocationTracker(invocationRepo)
	orchestrator := agent.NewOrchestrator(
		invocationRepo, threadRepo, messageRepo,
		configService, baseAgentService, tracker, workflowEngine, workflowRepo, projectRepo, wsHub, claudeAdapter,
	)

	// 连接Message服务和Agent编排器（用户消息触发Agent）
	messageService.SetAgentSpawner(orchestrator)

	gatekeeper := merge.NewGatekeeper(reviewRepo, artifactRepo, threadRepo)

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
	router.Use(requestLogger(logger))

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

	// 先注册 invocationHandler（包含 /threads/:id/invocations）
	invocationHandler := api.NewInvocationHandler(orchestrator, mcpAuthService, projectRepo)
	invocationHandler.RegisterRoutes(v1)

	// Artifact Handler（包含 /threads/:id/artifacts）
	artifactHandler := api.NewArtifactHandler(artifactRepo)
	artifactHandler.RegisterRoutes(v1)

	// Merge Handler（包含 /threads/:id/merge/*）
	mergeHandler := api.NewMergeHandler(gatekeeper)
	mergeHandler.RegisterRoutes(v1)

	// 再注册 threadHandler（包含 /threads/:id）
	threadHandler := api.NewThreadHandler(threadService)
	threadHandler.RegisterRoutes(v1)

	messageHandler := api.NewMessageHandler(messageService)
	messageHandler.RegisterRoutes(v1)

	agentHandler := api.NewAgentHandler(configService, baseAgentService, orchestrator, threadRepo)
	agentHandler.RegisterRoutes(v1)

	// 基础Agent Handler
	baseAgentHandler := api.NewBaseAgentHandler(baseAgentService)
	baseAgentHandler.RegisterRoutes(v1)

	// 工作流模板 Handler
	workflowHandler := api.NewWorkflowHandler(workflowService)
	workflowHandler.RegisterRoutes(v1)

	// WebSocket
	wsHandler := ws.NewHandler(wsHub)
	wsHandler.RegisterRoutes(v1)

	// 沙箱 Handler (如果可用)
	if sandboxService != nil {
		sandboxHandler := api.NewSandboxHandler(sandboxService)
		sandboxHandler.RegisterRoutes(v1)
	}

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
	// 确保日志目录存在
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// 配置 lumberjack 用于日志轮转
	logFile := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "server.log"),
		MaxSize:    10,   // MB，单个文件最大大小
		MaxBackups: 5,    // 保留的旧日志文件数量
		MaxAge:     30,   // 天，保留天数
		Compress:   true, // 压缩旧日志文件
	}

	// 解析日志级别
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	// 配置编码器
	var encoder zapcore.Encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	if format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 创建多输出：同时输出到文件和控制台
	fileWriter := zapcore.AddSync(logFile)
	consoleWriter := zapcore.AddSync(os.Stdout)

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, fileWriter, zapLevel),
		zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), consoleWriter, zapLevel),
	)

	return zap.New(core, zap.AddCaller()), nil
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

// requestLogger 请求日志中间件
func requestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		if status >= 400 {
			logger.Error("HTTP Request",
				zap.String("method", c.Request.Method),
				zap.String("path", path),
				zap.String("query", query),
				zap.Int("status", status),
				zap.Duration("latency", latency),
				zap.String("client_ip", c.ClientIP()),
			)
		} else {
			logger.Info("HTTP Request",
				zap.String("method", c.Request.Method),
				zap.String("path", path),
				zap.String("query", query),
				zap.Int("status", status),
				zap.Duration("latency", latency),
				zap.String("client_ip", c.ClientIP()),
			)
		}
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
    local_path TEXT NOT NULL DEFAULT '',
    git_repo TEXT,
    config TEXT,
    workflow_template_id TEXT,
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
    workflow_template_id TEXT,
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

-- 基础Agent配置表
CREATE TABLE IF NOT EXISTS base_agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    api_url TEXT,
    api_token TEXT,
    default_model TEXT,
    cli_path TEXT DEFAULT 'claude',
    git_bash_path TEXT,
    max_tokens INTEGER DEFAULT 4096,
    timeout_minutes INTEGER DEFAULT 30,
    is_active INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
    base_agent_id TEXT REFERENCES base_agents(id),
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

-- 工作流模板表
CREATE TABLE IF NOT EXISTS workflow_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    agent_ids TEXT DEFAULT '[]',
    checkpoints TEXT DEFAULT '[]',
    estimated_time TEXT,
    is_system INTEGER DEFAULT 0,
    is_default INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_threads_project_id ON threads(project_id);
CREATE INDEX IF NOT EXISTS idx_messages_thread_id ON messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_agent_invocations_thread_id ON agent_invocations(thread_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_thread_id ON artifacts(thread_id);
`
	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// 迁移：检查并添加新列
	migrations := []struct {
		table  string
		column string
		typ    string
	}{
		{"agent_configs", "base_agent_id", "TEXT"},
		{"projects", "workflow_template_id", "TEXT"},
		{"projects", "local_path", "TEXT NOT NULL DEFAULT ''"},
		{"threads", "workflow_template_id", "TEXT"},
		{"workflow_templates", "is_default", "INTEGER DEFAULT 0"},
		{"base_agents", "git_bash_path", "TEXT"},
	}

	for _, m := range migrations {
		var count int
		row := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name='%s'", m.table, m.column))
		if err := row.Scan(&count); err == nil && count == 0 {
			_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", m.table, m.column, m.typ))
			if err != nil {
				fmt.Printf("Warning: could not add %s column to %s: %v\n", m.column, m.table, err)
			}
		}
	}

	return nil
}