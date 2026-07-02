// Package agent — Prompt Builder (V2)
//
// 借鉴 clowder-ai：
//   - invoke-single-cat.ts:1682-1745 的 injectSystemPrompt 决策
//   - StagingContent.ts:239-258 的 buildStagingPrepend 独立通道（ADR-038）
//   - AcpAgentService.ts:186-284 的 resumeFallbackSystemPrompt 语义
//
// 核心不变式：
//  1. Staging 通道**每轮必发**，与 systemPrompt 注入决策独立（"每轮注入生效"契约）
//  2. Resume + canSkipOnResume 时**跳过** Layer0 / Environment / Memory
//     —— CLI transcript 里已保留过，Memory 可靠 MCP 拉取
//  3. Force / Registry Changed / 首次 → 全量注入
//  4. A2A upstream handoff 独立于 mode，永远注入（如果有）
package agent

import "strings"

// PromptMode 决定 Prompt 组装策略
type PromptMode int

const (
	// PromptModeNew 首次拉起：全量注入 Layer0 / Env / Memory
	PromptModeNew PromptMode = iota
	// PromptModeResume 使用 --resume 恢复：跳过静态部分，只发 user input + 短 env
	PromptModeResume
	// PromptModeForceRefresh Registry 变更 / 压缩检测触发 / 手工刷新：像 New 一样全量注入
	PromptModeForceRefresh
)

// String 便于日志
func (m PromptMode) String() string {
	switch m {
	case PromptModeNew:
		return "new"
	case PromptModeResume:
		return "resume"
	case PromptModeForceRefresh:
		return "force_refresh"
	default:
		return "unknown"
	}
}

// PromptBuildRequest V2 组装请求
//
// 与旧 ExecutionRequest.Context 解耦：调用方（execution_service）自己算好 Mode。
type PromptBuildRequest struct {
	Mode PromptMode

	// Layers 复用旧的四层结构；Layer0/Layer2/Layer3/Memory 是否发出由 Mode 决定
	Layers *ContextLayers

	// StagingPrefix 每轮独立通道（ADR-038 借鉴）
	// 内容示例：动态 workflow 阶段提醒、governance 补丁、compression hint
	// 空串表示本轮无 staging
	StagingPrefix string

	// UpstreamHandoff 上游 Agent 的 <a2a-handoff> 块结构化内容（不含标签）
	// 空串表示不是 A2A 或没抓到 handoff（此时 caller 应用短 chain history 兜底）
	UpstreamHandoff string
	// UpstreamName 上游 Agent 名字，用于给 handoff 加标题
	UpstreamName string

	// Input 用户实际输入
	Input string

	// EnvShortLine Resume 场景专用：极短 env 状态行（<50 tokens）
	// 例："env: phase=dev, agent=xxx, chain=3/10"
	// 由 caller 组装，BuildPromptV2 只透传
	EnvShortLine string
}

// BuildPromptV2 按 Mode 组装 prompt
//
// 顺序（借鉴 clowder-ai invoke-single-cat.ts:1715-1745）：
//
//	stagingPrepend                (每轮必发)
//	<system>                      (New/Force 才发)
//	<environment>                 (New/Force 才发)
//	<memory>                      (New/Force 才发)
//	env-short-line                (Resume 才发)
//	<a2a-handoff-from-upstream>   (A2A 时必发，紧邻 user input —— "上游说的话"就是本轮输入的直接前置)
//	<user> / <a2a_input>          (永远发)
func BuildPromptV2(req *PromptBuildRequest) string {
	if req == nil {
		return ""
	}

	var sb strings.Builder
	fullInject := req.Mode == PromptModeNew || req.Mode == PromptModeForceRefresh

	// 1) Staging —— 每轮独立通道，不受 mode 影响
	//    注意：绝对不要合并进 systemPrompt，否则 Resume 会一起丢
	if req.StagingPrefix != "" {
		sb.WriteString(req.StagingPrefix)
		sb.WriteString("\n\n---\n\n")
	}

	if req.Layers != nil {
		// 2) Layer0 SystemPrompt —— 仅 New / ForceRefresh
		//    Resume 场景 CLI transcript 里已有，重发就是浪费
		if fullInject && req.Layers.Layer0 != "" {
			sb.WriteString("<system>\n")
			sb.WriteString(req.Layers.Layer0)
			sb.WriteString("\n</system>\n\n")
		}

		// 3) Layer2 Artifacts —— 仅 New / ForceRefresh
		if fullInject && req.Layers.Layer2 != "" {
			sb.WriteString("<artifacts>\n")
			sb.WriteString(req.Layers.Layer2)
			sb.WriteString("\n</artifacts>\n\n")
		}

		// 4) Layer3 Environment —— 仅 New / ForceRefresh
		if fullInject && req.Layers.Layer3 != "" {
			sb.WriteString("<environment>\n")
			sb.WriteString(req.Layers.Layer3)
			sb.WriteString("\n</environment>\n\n")
		}

		// 5) Memory —— 仅 New / ForceRefresh；Resume 场景走 isdp-memory MCP tool 拉取
		if fullInject && req.Layers.MemoryContext != "" {
			sb.WriteString("<memory>\n")
			sb.WriteString(req.Layers.MemoryContext)
			sb.WriteString("\n</memory>\n\n")
		}
	}

	// 6) Resume 场景补一行极短 env（<50 tokens）
	//    这是 clowder-ai D1/D7 语义的最小化 —— 每次都要发的动态锚点
	if !fullInject && req.EnvShortLine != "" {
		sb.WriteString(req.EnvShortLine)
		sb.WriteString("\n\n")
	}

	// 7) A2A upstream handoff —— 独立通道，只发上一手（O(1)，不是 O(N)）
	//    位置紧邻 user input：语义上"上游 Agent 的输出"是本轮 user prompt 的直接前置
	if req.UpstreamHandoff != "" {
		sb.WriteString("<a2a-handoff-from-upstream>\n")
		if req.UpstreamName != "" {
			sb.WriteString("[上游 Agent: ")
			sb.WriteString(req.UpstreamName)
			sb.WriteString("]\n\n")
		}
		sb.WriteString(req.UpstreamHandoff)
		sb.WriteString("\n</a2a-handoff-from-upstream>\n\n")
	}

	// 8) 用户输入
	tag := "user"
	if strings.Contains(req.Input, "## 协作规则") || strings.Contains(req.Input, "Direct message from") {
		tag = "a2a_input"
	}
	sb.WriteString("<")
	sb.WriteString(tag)
	sb.WriteString(">\n")
	sb.WriteString(req.Input)
	sb.WriteString("\n</")
	sb.WriteString(tag)
	sb.WriteString(">\n")

	return sb.String()
}

// ==============================================================================
// 向后兼容层
// ==============================================================================

// BuildPromptFromRequest 保留旧签名，内部默认走 PromptModeNew（无损兼容）
// 待调用点全部改造完毕后，此函数可标记 Deprecated 或删除。
func BuildPromptFromRequest(req *ExecutionRequest) string {
	if req == nil {
		return ""
	}
	return BuildPrompt(req.Context, req.Input)
}

// BuildPrompt 旧版本：按老规则组装（等价于 V2 的 PromptModeNew + Legacy A2A ChainHistory）
// 保留是为了让未升级到 V2 的调用点继续跑；
// **新代码请一律用 BuildPromptV2**。
func BuildPrompt(layers *ContextLayers, input string) string {
	var sb strings.Builder

	if layers != nil {
		if layers.Layer0 != "" {
			sb.WriteString("<system>\n")
			sb.WriteString(layers.Layer0)
			sb.WriteString("\n</system>\n\n")
		}

		if layers.ChainHistory != "" {
			sb.WriteString("<a2a-context>\n")
			sb.WriteString(layers.ChainHistory)
			sb.WriteString("\n</a2a-context>\n\n")
		}

		if layers.Layer2 != "" {
			sb.WriteString("<artifacts>\n")
			sb.WriteString(layers.Layer2)
			sb.WriteString("\n</artifacts>\n\n")
		}

		if layers.Layer3 != "" {
			sb.WriteString("<environment>\n")
			sb.WriteString(layers.Layer3)
			sb.WriteString("\n</environment>\n\n")
		}

		if layers.MemoryContext != "" {
			sb.WriteString("<memory>\n")
			sb.WriteString(layers.MemoryContext)
			sb.WriteString("\n</memory>\n\n")
		}
	}

	tag := "user"
	if strings.Contains(input, "## 协作规则") || strings.Contains(input, "Direct message from") {
		tag = "a2a_input"
	}

	sb.WriteString("<")
	sb.WriteString(tag)
	sb.WriteString(">\n")
	sb.WriteString(input)
	sb.WriteString("\n</")
	sb.WriteString(tag)
	sb.WriteString(">\n")

	return sb.String()
}
