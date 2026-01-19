package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func appendRequestPath(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if other == nil {
		return
	}
	if ctx != nil && ctx.Request != nil && ctx.Request.URL != nil {
		if path := ctx.Request.URL.Path; path != "" {
			other["request_path"] = path
			return
		}
	}
	if relayInfo != nil && relayInfo.RequestURLPath != "" {
		path := relayInfo.RequestURLPath
		if idx := strings.Index(path, "?"); idx != -1 {
			path = path[:idx]
		}
		other["request_path"] = path
	}
}

// GenerateTextOtherInfoByPrice 使用 Price 生成文本日志信息
func GenerateTextOtherInfoByPrice(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, spec *helper.PriceSpec, cacheTokens int) map[string]interface{} {
	other := make(map[string]interface{})

	if spec != nil {
		// 新版 Price 字段
		other["input_price"] = spec.InputPrice
		other["output_price"] = spec.OutputPrice
		other["cache_price"] = spec.CachePrice
		other["quota_type"] = spec.QuotaType
		if spec.QuotaType == 1 {
			other["fixed_price"] = spec.FixedPrice
			other["model_price"] = spec.FixedPrice // 兼容前端
		} else {
			other["model_price"] = -1 // 前端约定：按量计费时用 -1 表示
		}

		// 兼容旧版前端的 ratio 字段
		// 旧公式: inputPrice = modelRatio * 2.0，所以 modelRatio = inputPrice / 2.0
		modelRatio := spec.InputPrice / 2.0
		if modelRatio <= 0 {
			modelRatio = 1.0
		}
		other["model_ratio"] = modelRatio

		// completion_ratio = outputPrice / inputPrice
		completionRatio := 1.0
		if spec.InputPrice > 0 && spec.OutputPrice > 0 {
			completionRatio = spec.OutputPrice / spec.InputPrice
		}
		other["completion_ratio"] = completionRatio

		// cache_ratio = cachePrice / inputPrice
		cacheRatio := 1.0
		if spec.InputPrice > 0 && spec.CachePrice > 0 {
			cacheRatio = spec.CachePrice / spec.InputPrice
		}
		other["cache_ratio"] = cacheRatio

		// group_ratio 在新系统中始终为 1（分组价格差异已在 group_model_prices 表中体现）
		other["group_ratio"] = 1.0
	}
	other["cache_tokens"] = cacheTokens
	other["frt"] = float64(relayInfo.FirstResponseTime.UnixMilli() - relayInfo.StartTime.UnixMilli())

	if relayInfo.ReasoningEffort != "" {
		other["reasoning_effort"] = relayInfo.ReasoningEffort
	}
	if relayInfo.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = relayInfo.UpstreamModelName
	}

	isSystemPromptOverwritten := common.GetContextKeyBool(ctx, constant.ContextKeySystemPromptOverride)
	if isSystemPromptOverwritten {
		other["is_system_prompt_overwritten"] = true
	}

	adminInfo := make(map[string]interface{})
	adminInfo["use_channel"] = ctx.GetStringSlice("use_channel")
	isMultiKey := common.GetContextKeyBool(ctx, constant.ContextKeyChannelIsMultiKey)
	if isMultiKey {
		adminInfo["is_multi_key"] = true
		adminInfo["multi_key_index"] = common.GetContextKeyInt(ctx, constant.ContextKeyChannelMultiKeyIndex)
	}

	isLocalCountTokens := common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens)
	if isLocalCountTokens {
		adminInfo["local_count_tokens"] = isLocalCountTokens
	}

	other["admin_info"] = adminInfo
	appendRequestPath(ctx, relayInfo, other)
	return other
}

// GenerateWssOtherInfoByPrice 使用 Price 生成 WebSocket 日志信息
func GenerateWssOtherInfoByPrice(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.RealtimeUsage, spec *helper.PriceSpec) map[string]interface{} {
	info := GenerateTextOtherInfoByPrice(ctx, relayInfo, spec, 0)
	info["ws"] = true
	info["audio_input"] = usage.InputTokenDetails.AudioTokens
	info["audio_output"] = usage.OutputTokenDetails.AudioTokens
	info["text_input"] = usage.InputTokenDetails.TextTokens
	info["text_output"] = usage.OutputTokenDetails.TextTokens
	if spec != nil {
		info["audio_input_price"] = spec.AudioInputPrice
		info["audio_output_price"] = spec.AudioOutputPrice
	}
	return info
}

// GenerateAudioOtherInfoByPrice 使用 Price 生成音频日志信息
func GenerateAudioOtherInfoByPrice(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, spec *helper.PriceSpec) map[string]interface{} {
	info := GenerateTextOtherInfoByPrice(ctx, relayInfo, spec, 0)
	info["audio"] = true
	info["audio_input"] = usage.PromptTokensDetails.AudioTokens
	info["audio_output"] = usage.CompletionTokenDetails.AudioTokens
	info["text_input"] = usage.PromptTokensDetails.TextTokens
	info["text_output"] = usage.CompletionTokenDetails.TextTokens
	if spec != nil {
		info["audio_input_price"] = spec.AudioInputPrice
		info["audio_output_price"] = spec.AudioOutputPrice
	}
	return info
}

// GenerateClaudeOtherInfoByPrice 使用 Price 生成 Claude 日志信息
func GenerateClaudeOtherInfoByPrice(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, spec *helper.PriceSpec,
	cacheTokens, cacheCreationTokens, cacheCreationTokens5m, cacheCreationTokens1h int) map[string]interface{} {
	info := GenerateTextOtherInfoByPrice(ctx, relayInfo, spec, cacheTokens)
	info["claude"] = true
	info["cache_creation_tokens"] = cacheCreationTokens
	if spec != nil {
		info["cache_creation_price"] = spec.CacheCreationPrice
	}
	if cacheCreationTokens5m != 0 {
		info["cache_creation_tokens_5m"] = cacheCreationTokens5m
	}
	if cacheCreationTokens1h != 0 {
		info["cache_creation_tokens_1h"] = cacheCreationTokens1h
	}
	return info
}

// GenerateMjOtherInfoByPrice 使用 Price 生成 MJ 日志信息
func GenerateMjOtherInfoByPrice(relayInfo *relaycommon.RelayInfo, priceData types.PerCallPriceData) map[string]interface{} {
	other := make(map[string]interface{})
	other["fixed_price"] = priceData.FixedPrice
	other["quota"] = priceData.Quota
	appendRequestPath(nil, relayInfo, other)
	return other
}
