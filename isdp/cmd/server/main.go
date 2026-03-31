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
	"github.com/anthropic/isdp/internal/middleware"
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/a2a"
	"github.com/anthropic/isdp/internal/service/assetpackage"
	"github.com/anthropic/isdp/internal/service/command"
	"github.com/anthropic/isdp/internal/service/configgen"
	"github.com/anthropic/isdp/internal/service/knowledge"
	"github.com/anthropic/isdp/internal/service/mention"
	"github.com/anthropic/isdp/internal/service/merge"
	"github.com/anthropic/isdp/internal/service/message"
	"github.com/anthropic/isdp/internal/service/project"
	"github.com/anthropic/isdp/internal/service/rule"
	"github.com/anthropic/isdp/internal/service/sandbox"
	"github.com/anthropic/isdp/internal/service/settings"
	"github.com/anthropic/isdp/internal/service/skill"
	"github.com/anthropic/isdp/internal/service/subagent"
	"github.com/anthropic/isdp/internal/service/thread"
	"github.com/anthropic/isdp/internal/service/workflow"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 版本信息（构建时通过 -ldflags 注入）
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// findConfigPath 查找配置文件路径，按优先级查找
func findConfigPath() string {
	// 1. 命令行参数 -config
	for i, arg := range os.Args {
		if arg == "-config" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}

	// 2. 环境变量 ISDP_CONFIG
	if envPath := os.Getenv("ISDP_CONFIG"); envPath != "" {
		return envPath
	}

	// 3. 安装后的配置路径
	installConfig := "data/configs/config.yaml"
	if _, err := os.Stat(installConfig); err == nil {
		return installConfig
	}

	// 4. 开发环境配置路径
	return "configs/config.yaml"
}

func main() {
	// 设置 Windows 控制台 UTF-8 编码，解决中文乱码问题
	os.Stdout.WriteString("\x1b[?65001h")
	os.Stderr.WriteString("\x1b[?65001h")

	// 确定配置文件路径，按优先级查找
	configPath := findConfigPath()

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config from %s: %v\n", configPath, err)
		os.Exit(1)
	}
	fmt.Printf("Loaded config from: %s\n", configPath)

	// 初始化日志
	logger, err := initLogger(cfg.Logging.Level, cfg.Logging.Format, cfg.Data.GetLogsPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init logger: %v\n", err)
		os.Exit(1)
	}

	// 设置会话日志记录器
	agent.SetSessionLogger(logger)

	// 连接数据库
	db, dialect, err := repo.NewDB(cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// 初始化数据库表结构（仅SQLite需要，MySQL使用独立的初始化脚本）
	if cfg.Database.Type == config.DBTypeSQLite {
		if err := initDatabase(db); err != nil {
			logger.Fatal("Failed to initialize database", zap.Error(err))
		}
	}
	_ = dialect // 方言对象保留供后续使用

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
	ws.SetWSLogger(logger) // 设置 WebSocket 日志记录器

	// 初始化调试线程管理器
	debugThreadMgr := agent.NewDebugThreadManager(wsHub)
	defer debugThreadMgr.Stop() // 优雅关闭调试线程管理器

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
	skillRepo := repo.NewSkillRepository(db)
	agentSkillBindingRepo := repo.NewAgentSkillBindingRepository(db)
	registryRepo := repo.NewSkillRegistryRepository(db)
	knowledgeRepo := repo.NewKnowledgeBaseRepository(db)
	subagentRepo := repo.NewSubagentRepository(db)
	agentSubagentBindingRepo := repo.NewAgentSubagentBindingRepository(db)
	// Command 和 Rule 相关 Repositories
	commandRepo := repo.NewCommandRepository(db)
	ruleRepo := repo.NewRuleRepository(db)
	commandSkillBindingRepo := repo.NewCommandSkillBindingRepository(db)
	agentCommandBindingRepo := repo.NewAgentCommandBindingRepository(db)
	agentRuleBindingRepo := repo.NewAgentRuleBindingRepository(db)
	subagentSkillBindingRepo := repo.NewSubagentSkillBindingRepository(db)
	// Settings 和 AssetPackage 相关 Repositories
	settingsRepo := repo.NewSettingsRepository(db)
	agentSettingsBindingRepo := repo.NewAgentSettingsBindingRepository(db)
	assetPackageRepo := repo.NewAssetPackageRepository(db)

	// 初始化Services
	projectService := project.NewService(projectRepo, workflowRepo)
	threadService := thread.NewService(threadRepo, projectRepo, workflowRepo)
	messageService := message.NewService(messageRepo, wsHub)
	configService := agent.NewConfigService(agentConfigRepo)
	baseAgentService := agent.NewBaseAgentService(baseAgentRepo)
	workflowEngine := agent.NewWorkflowEngine(threadRepo, messageRepo, configService)
	workflowService := workflow.NewService(workflowRepo)
	mcpAuthService := a2a.NewMCPAuthService(cfg.MCP.TokenTTL)
	invocationRegistry := a2a.NewInvocationRegistry()  // 新增：调用注册表
	invocationQueue := a2a.NewInvocationQueue()       // 新增：请求队列

	// 初始化 Mention 模式注册表和解析器（支持动态 patterns 和博弈场景）
	mentionPatternRegistry := mention.NewPatternRegistry(agentConfigRepo)
	mentionParser := mention.NewParser(mentionPatternRegistry)

	// 刷新 mention patterns（从数据库加载）
	if err := mentionPatternRegistry.Refresh(context.Background()); err != nil {
		logger.Warn("Failed to refresh mention patterns", zap.Error(err))
	}

	skillService := skill.NewService(
		skillRepo, agentSkillBindingRepo, agentConfigRepo,
		subagentSkillBindingRepo, commandSkillBindingRepo,
		subagentRepo, commandRepo,
		cfg.GetSkillStoragePath(), logger,
	)
	registryService := skill.NewRegistryService(registryRepo, skillRepo)
	knowledgeService := knowledge.NewService(knowledgeRepo)
	configGenService := configgen.NewService(
		projectRepo, agentConfigRepo, skillRepo, agentSkillBindingRepo,
		subagentRepo, agentSubagentBindingRepo,
		commandRepo, ruleRepo,
		agentCommandBindingRepo, agentRuleBindingRepo,
		commandSkillBindingRepo, subagentSkillBindingRepo,
		cfg.GetSkillStoragePath(), cfg.GetSubagentStoragePath(),
		cfg.GetCommandStoragePath(), cfg.GetRuleStoragePath(),
		cfg.AgentConfig.DataDir,
		logger,
	)

	// 创建 Subagent Service
	subagentSvc := subagent.NewService(
		subagentRepo, agentSubagentBindingRepo, subagentSkillBindingRepo,
		agentConfigRepo, skillRepo,
		cfg.GetSubagentStoragePath(),
		logger,
	)

	// 创建 Command Service
	commandSvc := command.NewService(
		commandRepo, commandSkillBindingRepo, agentCommandBindingRepo,
		agentConfigRepo, skillRepo,
		cfg.GetCommandStoragePath(),
		logger,
	)

	// 创建 Rule Service
	ruleSvc := rule.NewService(
		ruleRepo, agentRuleBindingRepo, agentConfigRepo,
		cfg.GetRuleStoragePath(),
		logger,
	)

	// 创建 Settings Service
	settingsSvc := settings.NewService(
		settingsRepo, agentSettingsBindingRepo, agentConfigRepo,
		cfg.GetSettingsStoragePath(),
		logger,
	)

	// 创建 AssetPackage Service
	assetPackageSvc := assetpackage.NewService(
		assetPackageRepo, skillRepo, commandRepo, subagentRepo, ruleRepo, settingsRepo,
		settingsSvc,
		commandSkillBindingRepo, subagentSkillBindingRepo,
		cfg.GetSkillStoragePath(), cfg.GetSubagentStoragePath(),
		cfg.GetCommandStoragePath(), cfg.GetRuleStoragePath(),
		cfg.GetSettingsStoragePath(),
		logger,
	)

	// 初始化 UseCountUpdater（技能使用次数统计）
	useCountUpdater := skill.NewUseCountUpdater(skillRepo, projectRepo, agentSkillBindingRepo)
	useCountUpdater.SetWorkflowService(workflowService)
	useCountUpdater.SetLogger(logger)

	// 启动技能使用次数统计定时任务
	updateInterval, err := time.ParseDuration(cfg.Skill.UseCountUpdateInterval)
	if err != nil {
		updateInterval = time.Hour // 默认 1 小时
	}
	useCountUpdater.Start(updateInterval)
	defer useCountUpdater.Stop()

	// 初始化默认基础Agent
	if err := baseAgentService.InitDefaultAgents(context.Background()); err != nil {
		logger.Warn("Failed to initialize default base agents", zap.Error(err))
	}

	// 初始化系统预置Agent角色
	if err := configService.InitSystemAgents(context.Background()); err != nil {
		logger.Warn("Failed to initialize system agents", zap.Error(err))
	}

	// 初始化系统工作流模板
	if err := workflowService.InitSystemTemplates(context.Background()); err != nil {
		logger.Warn("Failed to initialize system workflow templates", zap.Error(err))
	}

	// 初始化适配器
	// 默认适配器设为 nil，在执行时根据 AgentRoleConfig.BaseAgentID 动态创建适配器
	// 这样可以支持多种类型的 Agent（Claude、OpenCode 等）
	var defaultAdapter agent.AgentAdapter = nil
	_ = agent.NewContextBuilder(threadRepo, messageRepo, artifactRepo) // TODO: wire into orchestrator
	tracker := agent.NewInvocationTracker(invocationRepo)
	orchestrator := agent.NewOrchestrator(
		invocationRepo, threadRepo, messageRepo,
		configService, baseAgentService, baseAgentRepo, tracker, workflowEngine, workflowRepo, projectRepo, wsHub, defaultAdapter, mentionParser,
	)

	// 在Orchestrator中设置调试管理器
	orchestrator.SetDebugThreadManager(debugThreadMgr)

	// 连接Message服务和Agent编排器（用户消息触发Agent）
	messageService.SetAgentSpawner(orchestrator)

	// 创建队列处理器（在 orchestrator 创建之后）
	queueProcessor := a2a.NewQueueProcessor(a2a.QueueProcessorDeps{
		Queue:    invocationQueue,
		Registry: invocationRegistry,
		WSHub:    wsHub,
		SpawnAgent: func(ctx context.Context, threadID uuid.UUID, catID string, content string) error {
			// 通过 Orchestrator 触发 Agent
			req := &agent.SpawnRequest{
				ThreadID: threadID,
				Role:     getRoleFromCatID(catID),
				Input:    content,
			}
			_, err := orchestrator.SpawnAgent(ctx, req)
			return err
		},
	})
	_ = queueProcessor // TODO: wire into orchestrator completion hooks

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

	// 邀请码验证中间件
	inviteMiddleware := middleware.NewInviteMiddleware(cfg.Auth.InviteCode)
	router.Use(inviteMiddleware.Handler())

	// 邀请码验证接口
	router.POST("/api/v1/auth/invite", inviteMiddleware.VerifyInvite)

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"version":    Version,
			"gitCommit":  GitCommit,
			"buildTime":  BuildTime,
			"time":       time.Now().Format(time.RFC3339),
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

	agentHandler := api.NewAgentHandler(configService, baseAgentService, orchestrator, threadRepo, debugThreadMgr, workflowRepo,
		agentSkillBindingRepo, agentSubagentBindingRepo, agentCommandBindingRepo, agentRuleBindingRepo)
	agentHandler.RegisterRoutes(v1)

	// 基础Agent Handler
	baseAgentHandler := api.NewBaseAgentHandler(baseAgentService)
	baseAgentHandler.RegisterRoutes(v1)

	// 工作流模板 Handler
	workflowHandler := api.NewWorkflowHandler(workflowService)
	workflowHandler.RegisterRoutes(v1)

	// Skill Handler
	skillHandler := api.NewSkillHandler(skillService, cfg.GetSkillStoragePath(), cfg.Skill.GetUploadMaxSize())
	skillHandler.RegisterRoutes(v1)

	// Registry Handler
	registryHandler := api.NewRegistryHandler(registryService)
	registryHandler.RegisterRoutes(v1)

	// Knowledge Handler
	knowledgeHandler := api.NewKnowledgeHandler(knowledgeService)
	knowledgeHandler.RegisterRoutes(v1)

	// Subagent Handler
	subagentHandler := api.NewSubagentHandler(subagentSvc, cfg.GetSubagentStoragePath(), cfg.Subagent.GetUploadMaxSize())
	subagentHandler.RegisterRoutes(v1)

	// ConfigGen Handler
	configGenHandler := api.NewConfigGenHandler(configGenService)
	configGenHandler.RegisterRoutes(v1)

// Command Handler
	commandHandler := api.NewCommandHandler(commandSvc, cfg.GetCommandStoragePath(), cfg.Command.GetUploadMaxSize())
	commandHandler.RegisterRoutes(v1)

	// Rule Handler
	ruleHandler := api.NewRuleHandler(ruleSvc, cfg.GetRuleStoragePath(), cfg.Rule.GetUploadMaxSize())
	ruleHandler.RegisterRoutes(v1)

	// Settings Handler
	settingsHandler := api.NewSettingsHandler(settingsSvc, cfg.GetSettingsStoragePath())
	settingsHandler.RegisterRoutes(v1)

	// AssetPackage Handler
	assetPackageHandler := api.NewAssetPackageHandler(assetPackageSvc)
	assetPackageHandler.RegisterRoutes(v1)

	// MCP Callback Handler
	callbackHandler := api.NewCallbackHandler(invocationRegistry, mcpAuthService, messageService, messageRepo, wsHub, orchestrator, baseAgentRepo, invocationQueue, queueProcessor, mentionParser)
	callbackHandler.RegisterRoutes(v1)

	// WebSocket
	wsHandler := ws.NewHandler(wsHub)
	wsHandler.RegisterRoutes(v1)

	// 沙箱 Handler (如果可用)
	if sandboxService != nil {
		sandboxHandler := api.NewSandboxHandler(sandboxService)
		sandboxHandler.RegisterRoutes(v1)
	}

	// 前端静态文件服务
	router.Static("/assets", "./web/assets")
	router.StaticFile("/favicon.svg", "./web/favicon.svg")

	// SPA fallback - 所有未匹配的路由返回 index.html
	router.NoRoute(func(c *gin.Context) {
		c.File("./web/index.html")
	})

	// 启动服务器
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// 启动定期日志维护任务
	go func() {
		ticker := time.NewTicker(24 * time.Hour) // 每24小时检查一次
		defer ticker.Stop()

		// 立即执行一次日志维护
		performLogMaintenance(logger, cfg.Data.GetLogsPath())

		for {
			select {
			case <-ticker.C:
				// 定期执行日志维护
				performLogMaintenance(logger, cfg.Data.GetLogsPath())
			}
		}
	}()

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

func initLogger(level, format, logDir string) (*zap.Logger, error) {
	// 确保日志目录存在
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

-- Command 和 Rule 相关索引
CREATE INDEX IF NOT EXISTS idx_commands_name ON commands(name);
CREATE INDEX IF NOT EXISTS idx_rules_scope ON rules(scope);
CREATE INDEX IF NOT EXISTS idx_agent_command_bindings_agent_role_id ON agent_command_bindings(agent_role_id);
CREATE INDEX IF NOT EXISTS idx_agent_command_bindings_command_id ON agent_command_bindings(command_id);
CREATE INDEX IF NOT EXISTS idx_agent_rule_bindings_agent_role_id ON agent_rule_bindings(agent_role_id);
CREATE INDEX IF NOT EXISTS idx_agent_rule_bindings_rule_id ON agent_rule_bindings(rule_id);
CREATE INDEX IF NOT EXISTS idx_command_skill_bindings_command_id ON command_skill_bindings(command_id);
CREATE INDEX IF NOT EXISTS idx_command_skill_bindings_skill_id ON command_skill_bindings(skill_id);
CREATE INDEX IF NOT EXISTS idx_subagent_skill_bindings_subagent_id ON subagent_skill_bindings(subagent_id);
CREATE INDEX IF NOT EXISTS idx_subagent_skill_bindings_skill_id ON subagent_skill_bindings(skill_id);

-- 命令表
CREATE TABLE IF NOT EXISTS commands (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 规约表
CREATE TABLE IF NOT EXISTS rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    scope TEXT NOT NULL DEFAULT 'instance',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Agent-Command 绑定表
CREATE TABLE IF NOT EXISTS agent_command_bindings (
    id TEXT PRIMARY KEY,
    agent_role_id TEXT NOT NULL,
    command_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, command_id)
);

-- Agent-Rule 绑定表
CREATE TABLE IF NOT EXISTS agent_rule_bindings (
    id TEXT PRIMARY KEY,
    agent_role_id TEXT NOT NULL,
    rule_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, rule_id)
);

-- Command-Skill 绑定表
CREATE TABLE IF NOT EXISTS command_skill_bindings (
    id TEXT PRIMARY KEY,
    command_id TEXT NOT NULL,
    skill_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(command_id, skill_id)
);

-- Subagent-Skill 绑定表
CREATE TABLE IF NOT EXISTS subagent_skill_bindings (
    id TEXT PRIMARY KEY,
    subagent_id TEXT NOT NULL,
    skill_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(subagent_id, skill_id)
);
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

	// 迁移：移除 agent_configs 表的 model_name 字段（如果存在）
	// SQLite 不支持直接 DROP COLUMN，需要检查是否需要迁移
	var modelNameExists int
	row := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('agent_configs') WHERE name='model_name'")
	if err := row.Scan(&modelNameExists); err == nil && modelNameExists > 0 {
		// 需要迁移：重建表移除 model_name 字段
		fmt.Println("Migrating agent_configs table: removing model_name column...")
		migrateSQL := `
		CREATE TABLE IF NOT EXISTS agent_configs_new (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			role TEXT NOT NULL,
			description TEXT,
			system_prompt TEXT,
			max_tokens INTEGER DEFAULT 4096,
			temperature REAL DEFAULT 0.7,
			routing_config TEXT,
			base_agent_id TEXT REFERENCES base_agents(id),
			is_default INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO agent_configs_new (id, name, role, description, system_prompt, max_tokens, temperature, routing_config, base_agent_id, is_default, created_at, updated_at)
		SELECT id, name, role, description, system_prompt, max_tokens, temperature, routing_config, base_agent_id, is_default, created_at, updated_at
		FROM agent_configs;
		DROP TABLE agent_configs;
		ALTER TABLE agent_configs_new RENAME TO agent_configs;
		CREATE INDEX IF NOT EXISTS idx_agent_configs_base_agent_id ON agent_configs(base_agent_id);
		`
		_, err = db.Exec(migrateSQL)
		if err != nil {
			fmt.Printf("Warning: could not migrate agent_configs table: %v\n", err)
		} else {
			fmt.Println("Successfully migrated agent_configs table")
		}
	}

	return nil
}

// performLogMaintenance 执行日志维护任务
func performLogMaintenance(logger *zap.Logger, logDir string) {
	// 检查日志目录是否存在
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return
	}

	// 获取日志目录中的所有文件
	files, err := os.ReadDir(logDir)
	if err != nil {
		logger.Error("Failed to read log directory", zap.Error(err))
		return
	}

	cleanedCount := 0
	totalSize := int64(0)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		// 计算文件大小
		totalSize += info.Size()

		// 删除超过30天的日志文件
		if time.Since(info.ModTime()).Hours() > 720 { // 30天 = 30 * 24小时
			filePath := filepath.Join(logDir, file.Name())
			if err := os.Remove(filePath); err == nil {
				logger.Info("Deleted old log file", zap.String("file", filePath))
				cleanedCount++
			}
		}
	}

	logger.Info("Log maintenance completed",
		zap.Int("files_cleaned", cleanedCount),
		zap.Int64("total_size_bytes", totalSize))
}

// getRoleFromCatID 从 catID 获取 AgentRole
// catID 就是 role（如 "backend_developer"），直接返回
func getRoleFromCatID(catID string) model.AgentRole {
	return model.AgentRole(catID)
}