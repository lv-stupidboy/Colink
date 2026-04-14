package main

import (
	"context"
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
	"github.com/anthropic/isdp/internal/service/a2a"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/assetpackage"
	"github.com/anthropic/isdp/internal/service/command"
	"github.com/anthropic/isdp/internal/service/configgen"
	"github.com/anthropic/isdp/internal/service/im"
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
	"github.com/anthropic/isdp/internal/service/teampackage"
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

	// 设置 SessionRecorder（用于记录 A2A 失败/成功会话）
	agent.SetSessionRecorder(a2a.NewSessionRecorderImpl())

	// 连接数据库（数据库表结构由安装器初始化，服务启动时不执行 schema 创建）
	db, dialect, err := repo.NewDB(cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()
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
	dbType := cfg.Database.Type
	projectRepo := repo.NewProjectRepository(db, dbType)
	threadRepo := repo.NewThreadRepository(db, dbType)
	messageRepo := repo.NewMessageRepository(db, dbType)
	agentConfigRepo := repo.NewAgentConfigRepository(db, dbType)
	baseAgentRepo := repo.NewBaseAgentRepository(db, dbType)
	invocationRepo := repo.NewAgentInvocationRepository(db, dbType)
	artifactRepo := repo.NewArtifactRepository(db, dbType)
	sandboxRepo := repo.NewSandboxRepository(db, dbType)
	reviewRepo := repo.NewReviewRepository(artifactRepo)
	workflowRepo := repo.NewWorkflowTemplateRepository(db, dbType)
	skillRepo := repo.NewSkillRepository(db, dbType)
	agentSkillBindingRepo := repo.NewAgentSkillBindingRepository(db, dbType)
	registryRepo := repo.NewSkillRegistryRepository(db, dbType)
	knowledgeRepo := repo.NewKnowledgeBaseRepository(db, dbType)
	subagentRepo := repo.NewSubagentRepository(db, dbType)
	agentSubagentBindingRepo := repo.NewAgentSubagentBindingRepository(db, dbType)
	// Command 和 Rule 相关 Repositories
	commandRepo := repo.NewCommandRepository(db, dbType)
	ruleRepo := repo.NewRuleRepository(db, dbType)
	commandSkillBindingRepo := repo.NewCommandSkillBindingRepository(db, dbType)
	agentCommandBindingRepo := repo.NewAgentCommandBindingRepository(db, dbType)
	agentRuleBindingRepo := repo.NewAgentRuleBindingRepository(db, dbType)
	subagentSkillBindingRepo := repo.NewSubagentSkillBindingRepository(db, dbType)
	// Settings 和 AssetPackage 相关 Repositories
	settingsRepo := repo.NewSettingsRepository(db, dbType)
	agentSettingsBindingRepo := repo.NewAgentSettingsBindingRepository(db, dbType)
	// 后台执行支持：内容块持久化
	contentBlockRepo := repo.NewContentBlockRepository(db, dbType)

	// 初始化Services
	projectService := project.NewService(projectRepo, workflowRepo)
	threadService := thread.NewService(threadRepo, projectRepo, workflowRepo)
	messageService := message.NewService(messageRepo, wsHub)
	configService := agent.NewConfigService(agentConfigRepo)
	baseAgentService := agent.NewBaseAgentService(baseAgentRepo)
	workflowEngine := agent.NewWorkflowEngine(threadRepo, messageRepo, configService)
	workflowService := workflow.NewService(workflowRepo)
	mcpAuthService := a2a.NewMCPAuthService(cfg.MCP.TokenTTL)
	invocationRegistry := a2a.NewInvocationRegistry() // 新增：调用注册表
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
		settingsRepo, agentSettingsBindingRepo,
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
		skillRepo, commandRepo, subagentRepo, ruleRepo, settingsRepo,
		settingsSvc,
		commandSkillBindingRepo, subagentSkillBindingRepo,
		cfg.GetSkillStoragePath(), cfg.GetSubagentStoragePath(),
		cfg.GetCommandStoragePath(), cfg.GetRuleStoragePath(),
		cfg.GetSettingsStoragePath(),
		logger,
	)

	// 创建 TeamPackage Service
	teamPackageSvc := teampackage.NewService(
		workflowRepo, agentConfigRepo,
		skillRepo, commandRepo, subagentRepo, ruleRepo, settingsRepo,
		agentSkillBindingRepo, agentCommandBindingRepo, agentSubagentBindingRepo,
		agentRuleBindingRepo, agentSettingsBindingRepo,
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

	// 初始化适配器
	// 默认适配器设为 nil，在执行时根据 AgentRoleConfig.BaseAgentID 动态创建适配器
	// 这样可以支持多种类型的 Agent（Claude、OpenCode 等）
	var defaultAdapter agent.AgentAdapter = nil
	_ = agent.NewContextBuilder(threadRepo, messageRepo, artifactRepo) // TODO: wire into orchestrator
	tracker := agent.NewInvocationTracker(invocationRepo)
	orchestrator := agent.NewOrchestrator(
		invocationRepo, threadRepo, messageRepo,
		configService, baseAgentService, baseAgentRepo, tracker, workflowEngine, workflowRepo, projectRepo, wsHub, defaultAdapter, mentionParser,
		contentBlockRepo,
	)

	// 在Orchestrator中设置调试管理器
	orchestrator.SetDebugThreadManager(debugThreadMgr)

	// 启动恢复：检测并标记孤儿 invocation（后台执行支持）
	startupReconciler := agent.NewStartupReconciler(invocationRepo, contentBlockRepo)
	startupReconciler.Reconcile(context.Background())

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

	// ========== IM Integration ==========
	// Merge im.platforms feishu config into legacy cfg.Feishu if present
	for _, p := range cfg.IM.Platforms {
		if p.Type == "feishu" && p.Enabled {
			cfg.Feishu.Enabled = true
			cfg.Feishu.AppID = p.AppID
			cfg.Feishu.AppSecret = p.AppSecret
			cfg.Feishu.VerificationToken = p.VerificationToken
			cfg.Feishu.EncryptKey = p.EncryptKey
			cfg.Feishu.LarkCLIPath = p.LarkCLIPath
			cfg.Feishu.DefaultProjectID = p.DefaultProjectID
			cfg.Feishu.EventMode = p.EventMode
			break
		}
	}

	var imBridgeSvc *im.IMBridgeService
	var eventListener *im.EventListener
	if cfg.Feishu.Enabled {
		imSessionRepo := repo.NewIMSessionRepository(db)
		larkClient := im.NewLarkCLIClient(cfg.Feishu.LarkCLIPath, logger)
		feishuAdapter := im.NewFeishuAdapter(larkClient, logger)
		if err := feishuAdapter.CheckHealth(context.Background()); err != nil {
			logger.Warn("Feishu adapter health check failed", zap.Error(err))
		}
		rateLimiter := im.NewRateLimiter(20, 60*time.Second)
		dedupCache := im.NewDedupCache(1000)
		feishuDelivery := im.NewDeliveryService(feishuAdapter, im.DefaultRetryConfig(), rateLimiter, dedupCache, logger)
		imBridgeSvc = im.NewIMBridgeService(imSessionRepo, threadRepo, projectRepo, orchestrator, wsHub, nil, logger)
		imBridgeSvc.RegisterAdapter(feishuAdapter, feishuDelivery)
		orchestrator.GetExecutionService().AddChunkListener(imBridgeSvc.OnAgentChunk)

		eventMode := cfg.Feishu.EventMode
		if eventMode == "" {
			eventMode = config.EventModeListener
		}

		switch eventMode {
		case config.EventModeListener:
			eventListener = im.NewEventListener(cfg.Feishu.LarkCLIPath, imBridgeSvc, logger)
			if err := eventListener.Start(context.Background()); err != nil {
				logger.Error("Failed to start IM event listener, falling back to webhook mode", zap.Error(err))
				eventListener = nil
			}
		case config.EventModeWebhook:
			logger.Info("IM event mode: webhook (requires public URL)")
		default:
			logger.Warn("Unknown event_mode, defaulting to listener", zap.String("event_mode", eventMode))
			eventListener = im.NewEventListener(cfg.Feishu.LarkCLIPath, imBridgeSvc, logger)
			if err := eventListener.Start(context.Background()); err != nil {
				logger.Error("Failed to start IM event listener", zap.Error(err))
				eventListener = nil
			}
		}
		logger.Info("IM integration enabled", zap.String("platform", "feishu"), zap.String("event_mode", eventMode))
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
			"status":    "ok",
			"version":   Version,
			"gitCommit": GitCommit,
			"buildTime": BuildTime,
			"time":      time.Now().Format(time.RFC3339),
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

	// TeamPackage Handler
	teamPackageHandler := api.NewTeamPackageHandler(teamPackageSvc)
	teamPackageHandler.RegisterRoutes(v1)

	// MCP Callback Handler
	callbackHandler := api.NewCallbackHandler(invocationRegistry, mcpAuthService, messageService, messageRepo, wsHub, orchestrator, baseAgentRepo, invocationQueue, queueProcessor, mentionParser)
	callbackHandler.RegisterRoutes(v1)

	// WebSocket
	wsHandler := ws.NewHandler(wsHub, orchestrator, orchestrator)
	wsHandler.RegisterRoutes(v1)

	// 飞书 Webhook Handler (仅 webhook 模式)
	if imBridgeSvc != nil && eventListener == nil {
		feishuHandler := api.NewFeishuWebhookHandler(imBridgeSvc, cfg.Feishu.VerificationToken)
		feishuHandler.RegisterRoutes(v1)
	}

	// 沙箱 Handler (如果可用)
	if sandboxService != nil {
		sandboxHandler := api.NewSandboxHandler(sandboxService)
		sandboxHandler.RegisterRoutes(v1)
	}

	// 前端静态文件服务
	router.Static("/assets", "./web/assets")
	router.StaticFile("/favicon.svg", "./web/favicon.svg")
	router.StaticFile("/favicon.ico", "./web/favicon.ico")

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

	if eventListener != nil {
		eventListener.Stop()
	}

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
