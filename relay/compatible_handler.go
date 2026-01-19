package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

func TextHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	textReq, ok := info.Request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.GeneralOpenAIRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(textReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	if request.WebSearchOptions != nil {
		c.Set("chat_completion_web_search_context_size", request.WebSearchOptions.SearchContextSize)
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	includeUsage := true
	// 判断用户是否需要返回使用情况
	if request.StreamOptions != nil {
		includeUsage = request.StreamOptions.IncludeUsage
	}

	// 如果不支持StreamOptions，将StreamOptions设置为nil
	if !info.SupportStreamOptions || !request.Stream {
		request.StreamOptions = nil
	} else {
		// 如果支持StreamOptions，且请求中没有设置StreamOptions，根据配置文件设置StreamOptions
		if constant.ForceStreamOption {
			request.StreamOptions = &dto.StreamOptions{
				IncludeUsage: true,
			}
		}
	}

	info.ShouldIncludeUsage = includeUsage

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	passThroughGlobal := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	if info.RelayMode == relayconstant.RelayModeChatCompletions &&
		!passThroughGlobal &&
		!info.ChannelSetting.PassThroughBodyEnabled &&
		shouldChatCompletionsViaResponses(info) {
		applySystemPromptIfNeeded(c, info, request)
		usage, newApiErr := chatCompletionsViaResponses(c, info, adaptor, request)
		if newApiErr != nil {
			return newApiErr
		}

		var containAudioTokens = usage.CompletionTokenDetails.AudioTokens > 0 || usage.PromptTokensDetails.AudioTokens > 0
		// 检查模型是否配置了音频价格
		spec, _ := helper.GetPriceSpec("", info.OriginModelName)
		var containsAudioRatios = spec != nil && (spec.AudioInputPrice > 0 || spec.AudioOutputPrice > 0)

		if containAudioTokens && containsAudioRatios {
			service.PostAudioConsumeQuota(c, info, usage, "")
		} else {
			postConsumeQuota(c, info, usage)
		}
		return nil
	}

	var requestBody io.Reader

	if passThroughGlobal || info.ChannelSetting.PassThroughBodyEnabled {
		body, err := common.GetRequestBody(c)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		if common.DebugEnabled {
			println("requestBody: ", string(body))
		}
		requestBody = bytes.NewBuffer(body)
	} else {
		convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		if info.ChannelSetting.SystemPrompt != "" {
			// 如果有系统提示，则将其添加到请求中
			request, ok := convertedRequest.(*dto.GeneralOpenAIRequest)
			if ok {
				containSystemPrompt := false
				for _, message := range request.Messages {
					if message.Role == request.GetSystemRoleName() {
						containSystemPrompt = true
						break
					}
				}
				if !containSystemPrompt {
					// 如果没有系统提示，则添加系统提示
					systemMessage := dto.Message{
						Role:    request.GetSystemRoleName(),
						Content: info.ChannelSetting.SystemPrompt,
					}
					request.Messages = append([]dto.Message{systemMessage}, request.Messages...)
				} else if info.ChannelSetting.SystemPromptOverride {
					common.SetContextKey(c, constant.ContextKeySystemPromptOverride, true)
					// 如果有系统提示，且允许覆盖，则拼接到前面
					for i, message := range request.Messages {
						if message.Role == request.GetSystemRoleName() {
							if message.IsStringContent() {
								request.Messages[i].SetStringContent(info.ChannelSetting.SystemPrompt + "\n" + message.StringContent())
							} else {
								contents := message.ParseContent()
								contents = append([]dto.MediaContent{
									{
										Type: dto.ContentTypeText,
										Text: info.ChannelSetting.SystemPrompt,
									},
								}, contents...)
								request.Messages[i].Content = contents
							}
							break
						}
					}
				}
			}
		}

		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return types.NewError(err, types.ErrorCodeJsonMarshalFailed, types.ErrOptionWithSkipRetry())
		}

		// remove disabled fields for OpenAI API
		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// apply param override
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverride(jsonData, info.ParamOverride, relaycommon.BuildParamOverrideContext(info))
			if err != nil {
				return types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
			}
		}

		logger.LogDebug(c, fmt.Sprintf("text request body: %s", string(jsonData)))

		requestBody = bytes.NewBuffer(jsonData)
	}

	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	if resp != nil {
		httpResp = resp.(*http.Response)
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			newApiErr := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(newApiErr, statusCodeMappingStr)
			return newApiErr
		}
	}

	usage, newApiErr := adaptor.DoResponse(c, httpResp, info)
	if newApiErr != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newApiErr, statusCodeMappingStr)
		return newApiErr
	}

	var containAudioTokens = usage.(*dto.Usage).CompletionTokenDetails.AudioTokens > 0 || usage.(*dto.Usage).PromptTokensDetails.AudioTokens > 0
	// 检查模型是否配置了音频价格
	spec2, _ := helper.GetPriceSpec("", info.OriginModelName)
	var containsAudioRatios = spec2 != nil && (spec2.AudioInputPrice > 0 || spec2.AudioOutputPrice > 0)

	if containAudioTokens && containsAudioRatios {
		service.PostAudioConsumeQuota(c, info, usage.(*dto.Usage), "")
	} else {
		postConsumeQuota(c, info, usage.(*dto.Usage))
	}
	return nil
}

func shouldChatCompletionsViaResponses(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	if info.RelayMode != relayconstant.RelayModeChatCompletions {
		return false
	}
	return service.ShouldChatCompletionsUseResponsesGlobal(info.ChannelId, info.OriginModelName)
}

func postConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, extraContent ...string) {
	if usage == nil {
		usage = &dto.Usage{
			PromptTokens:     relayInfo.GetEstimatePromptTokens(),
			CompletionTokens: 0,
			TotalTokens:      relayInfo.GetEstimatePromptTokens(),
		}
		extraContent = append(extraContent, "上游无计费信息")
	}
	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	promptTokens := usage.PromptTokens
	cacheTokens := usage.PromptTokensDetails.CachedTokens
	imageTokens := usage.PromptTokensDetails.ImageTokens
	audioTokens := usage.PromptTokensDetails.AudioTokens
	completionTokens := usage.CompletionTokens
	cachedCreationTokens := usage.PromptTokensDetails.CachedCreationTokens

	modelName := relayInfo.OriginModelName
	tokenName := ctx.GetString("token_name")

	// 获取价格规格
	spec := relayInfo.PriceData.Spec
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
	dOneMillion := decimal.NewFromInt(1000000)

	// 计算各类 token 的配额（基于 Price 计费）
	// 公式: cost = (tokens / 1M) * price * QuotaPerUnit
	var quotaCalculateDecimal decimal.Decimal

	isClaudeUsageSemantic := relayInfo.ChannelType == constant.ChannelTypeAnthropic

	if spec.QuotaType == 1 {
		// 按次计费
		quotaCalculateDecimal = decimal.NewFromFloat(spec.FixedPrice).Mul(dQuotaPerUnit)
	} else {
		// 按量计费
		dPromptTokens := decimal.NewFromInt(int64(promptTokens))
		dCacheTokens := decimal.NewFromInt(int64(cacheTokens))
		dImageTokens := decimal.NewFromInt(int64(imageTokens))
		dAudioTokens := decimal.NewFromInt(int64(audioTokens))
		dCompletionTokens := decimal.NewFromInt(int64(completionTokens))
		dCachedCreationTokens := decimal.NewFromInt(int64(cachedCreationTokens))

		// 计算输入 tokens（扣除特殊 tokens）
		baseInputTokens := dPromptTokens
		// Anthropic API 的 input_tokens 已经不包含缓存 tokens，不需要减去
		// OpenAI/OpenRouter 等 API 的 prompt_tokens 包含缓存 tokens，需要减去
		if !dCacheTokens.IsZero() && !isClaudeUsageSemantic {
			baseInputTokens = baseInputTokens.Sub(dCacheTokens)
		}
		if !dCachedCreationTokens.IsZero() && !isClaudeUsageSemantic {
			baseInputTokens = baseInputTokens.Sub(dCachedCreationTokens)
		}
		if !dImageTokens.IsZero() {
			baseInputTokens = baseInputTokens.Sub(dImageTokens)
		}
		if !dAudioTokens.IsZero() {
			baseInputTokens = baseInputTokens.Sub(dAudioTokens)
		}

		// 文本输入配额
		inputCost := baseInputTokens.Div(dOneMillion).Mul(decimal.NewFromFloat(spec.InputPrice)).Mul(dQuotaPerUnit)
		// 文本输出配额
		outputCost := dCompletionTokens.Div(dOneMillion).Mul(decimal.NewFromFloat(spec.OutputPrice)).Mul(dQuotaPerUnit)
		// 缓存读取配额
		cacheCost := dCacheTokens.Div(dOneMillion).Mul(decimal.NewFromFloat(spec.CacheReadPrice)).Mul(dQuotaPerUnit)
		// 缓存创建配额
		cacheCreationCost := dCachedCreationTokens.Div(dOneMillion).Mul(decimal.NewFromFloat(spec.CacheWritePrice)).Mul(dQuotaPerUnit)
		// 图片配额
		imageCost := dImageTokens.Div(dOneMillion).Mul(decimal.NewFromFloat(spec.ImagePrice)).Mul(dQuotaPerUnit)
		// 音频输入配额
		audioCost := dAudioTokens.Div(dOneMillion).Mul(decimal.NewFromFloat(spec.AudioInputPrice)).Mul(dQuotaPerUnit)

		quotaCalculateDecimal = inputCost.Add(outputCost).Add(cacheCost).Add(cacheCreationCost).Add(imageCost).Add(audioCost)
	}

	// openai web search 工具计费
	var dWebSearchQuota decimal.Decimal
	var webSearchPrice float64
	if relayInfo.ResponsesUsageInfo != nil {
		if webSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool.CallCount > 0 {
			webSearchPrice = operation_setting.GetWebSearchPricePerThousand(modelName, webSearchTool.SearchContextSize)
			dWebSearchQuota = decimal.NewFromFloat(webSearchPrice).
				Mul(decimal.NewFromInt(int64(webSearchTool.CallCount))).
				Div(decimal.NewFromInt(1000)).Mul(dQuotaPerUnit)
			extraContent = append(extraContent, fmt.Sprintf("Web Search 调用 %d 次，上下文大小 %s，调用花费 %s",
				webSearchTool.CallCount, webSearchTool.SearchContextSize, dWebSearchQuota.String()))
		}
	} else if strings.HasSuffix(modelName, "search-preview") {
		searchContextSize := ctx.GetString("chat_completion_web_search_context_size")
		if searchContextSize == "" {
			searchContextSize = "medium"
		}
		webSearchPrice = operation_setting.GetWebSearchPricePerThousand(modelName, searchContextSize)
		dWebSearchQuota = decimal.NewFromFloat(webSearchPrice).
			Div(decimal.NewFromInt(1000)).Mul(dQuotaPerUnit)
		extraContent = append(extraContent, fmt.Sprintf("Web Search 调用 1 次，上下文大小 %s，调用花费 %s",
			searchContextSize, dWebSearchQuota.String()))
	}

	// claude web search tool 计费
	var dClaudeWebSearchQuota decimal.Decimal
	var claudeWebSearchPrice float64
	claudeWebSearchCallCount := ctx.GetInt("claude_web_search_requests")
	if claudeWebSearchCallCount > 0 {
		claudeWebSearchPrice = operation_setting.GetClaudeWebSearchPricePerThousand()
		dClaudeWebSearchQuota = decimal.NewFromFloat(claudeWebSearchPrice).
			Div(decimal.NewFromInt(1000)).Mul(dQuotaPerUnit).Mul(decimal.NewFromInt(int64(claudeWebSearchCallCount)))
		extraContent = append(extraContent, fmt.Sprintf("Claude Web Search 调用 %d 次，调用花费 %s",
			claudeWebSearchCallCount, dClaudeWebSearchQuota.String()))
	}

	// file search tool 计费
	var dFileSearchQuota decimal.Decimal
	var fileSearchPrice float64
	if relayInfo.ResponsesUsageInfo != nil {
		if fileSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolFileSearch]; exists && fileSearchTool.CallCount > 0 {
			fileSearchPrice = operation_setting.GetFileSearchPricePerThousand()
			dFileSearchQuota = decimal.NewFromFloat(fileSearchPrice).
				Mul(decimal.NewFromInt(int64(fileSearchTool.CallCount))).
				Div(decimal.NewFromInt(1000)).Mul(dQuotaPerUnit)
			extraContent = append(extraContent, fmt.Sprintf("File Search 调用 %d 次，调用花费 %s",
				fileSearchTool.CallCount, dFileSearchQuota.String()))
		}
	}

	// image generation call 计费
	var dImageGenerationCallQuota decimal.Decimal
	var imageGenerationCallPrice float64
	if ctx.GetBool("image_generation_call") {
		imageGenerationCallPrice = operation_setting.GetGPTImage1PriceOnceCall(ctx.GetString("image_generation_call_quality"), ctx.GetString("image_generation_call_size"))
		dImageGenerationCallQuota = decimal.NewFromFloat(imageGenerationCallPrice).Mul(dQuotaPerUnit)
		extraContent = append(extraContent, fmt.Sprintf("Image Generation Call 花费 %s", dImageGenerationCallQuota.String()))
	}

	// 添加工具调用配额
	quotaCalculateDecimal = quotaCalculateDecimal.Add(dWebSearchQuota)
	quotaCalculateDecimal = quotaCalculateDecimal.Add(dClaudeWebSearchQuota)
	quotaCalculateDecimal = quotaCalculateDecimal.Add(dFileSearchQuota)
	quotaCalculateDecimal = quotaCalculateDecimal.Add(dImageGenerationCallQuota)

	// 应用额外计费因子
	if len(relayInfo.PriceData.Multipliers) > 0 {
		for key, multiplier := range relayInfo.PriceData.Multipliers {
			dMultiplier := decimal.NewFromFloat(multiplier)
			quotaCalculateDecimal = quotaCalculateDecimal.Mul(dMultiplier)
			extraContent = append(extraContent, fmt.Sprintf("计费因子 %s: %f", key, multiplier))
		}
	}

	quota := int(quotaCalculateDecimal.Round(0).IntPart())
	totalTokens := promptTokens + completionTokens

	// 记录消费日志
	if totalTokens == 0 {
		quota = 0
		extraContent = append(extraContent, "上游没有返回计费信息，无法扣费（可能是上游超时）")
		logger.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s, pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, modelName, relayInfo.FinalPreConsumedQuota))
	} else {
		if quota == 0 && (spec.InputPrice > 0 || spec.OutputPrice > 0 || spec.FixedPrice > 0) {
			quota = 1
		}
		model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}

	quotaDelta := quota - relayInfo.FinalPreConsumedQuota

	if quotaDelta > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("预扣费后补扣费：%s（实际消耗：%s，预扣费：%s）",
			logger.FormatQuota(quotaDelta),
			logger.FormatQuota(quota),
			logger.FormatQuota(relayInfo.FinalPreConsumedQuota),
		))
	} else if quotaDelta < 0 {
		logger.LogInfo(ctx, fmt.Sprintf("预扣费后返还扣费：%s（实际消耗：%s，预扣费：%s）",
			logger.FormatQuota(-quotaDelta),
			logger.FormatQuota(quota),
			logger.FormatQuota(relayInfo.FinalPreConsumedQuota),
		))
	}

	if quotaDelta != 0 {
		err := service.PostConsumeQuota(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota, true)
		if err != nil {
			logger.LogError(ctx, "error consuming token remain quota: "+err.Error())
		}
	}

	logModel := modelName
	if strings.HasPrefix(logModel, "gpt-4-gizmo") {
		logModel = "gpt-4-gizmo-*"
		extraContent = append(extraContent, fmt.Sprintf("模型 %s", modelName))
	}
	if strings.HasPrefix(logModel, "gpt-4o-gizmo") {
		logModel = "gpt-4o-gizmo-*"
		extraContent = append(extraContent, fmt.Sprintf("模型 %s", modelName))
	}
	logContent := strings.Join(extraContent, ", ")

	// 构造 PriceSpec 用于日志
	logSpec := &helper.PriceSpec{
		InputPrice:         spec.InputPrice,
		OutputPrice:        spec.OutputPrice,
		CachePrice:         spec.CacheReadPrice,
		CacheCreationPrice: spec.CacheWritePrice,
		ImagePrice:         spec.ImagePrice,
		AudioInputPrice:    spec.AudioInputPrice,
		AudioOutputPrice:   spec.AudioOutputPrice,
		FixedPrice:         spec.FixedPrice,
		QuotaType:          spec.QuotaType,
	}
	other := service.GenerateTextOtherInfoByPrice(ctx, relayInfo, logSpec, cacheTokens)

	// Claude 模型特殊标记
	if isClaudeUsageSemantic {
		other["claude"] = true
		other["usage_semantic"] = "anthropic"
	}
	if imageTokens != 0 {
		other["image"] = true
		other["image_price"] = spec.ImagePrice
		other["image_tokens"] = imageTokens
	}
	if cachedCreationTokens != 0 {
		other["cache_creation_tokens"] = cachedCreationTokens
		other["cache_creation_price"] = spec.CacheWritePrice
	}
	if !dWebSearchQuota.IsZero() {
		if relayInfo.ResponsesUsageInfo != nil {
			if webSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists {
				other["web_search"] = true
				other["web_search_call_count"] = webSearchTool.CallCount
				other["web_search_price"] = webSearchPrice
			}
		} else if strings.HasSuffix(modelName, "search-preview") {
			other["web_search"] = true
			other["web_search_call_count"] = 1
			other["web_search_price"] = webSearchPrice
		}
	} else if !dClaudeWebSearchQuota.IsZero() {
		other["web_search"] = true
		other["web_search_call_count"] = claudeWebSearchCallCount
		other["web_search_price"] = claudeWebSearchPrice
	}
	if !dFileSearchQuota.IsZero() && relayInfo.ResponsesUsageInfo != nil {
		if fileSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolFileSearch]; exists {
			other["file_search"] = true
			other["file_search_call_count"] = fileSearchTool.CallCount
			other["file_search_price"] = fileSearchPrice
		}
	}
	if audioTokens > 0 && spec.AudioInputPrice > 0 {
		other["audio_input_seperate_price"] = true
		other["audio_input_token_count"] = audioTokens
		other["audio_input_price"] = spec.AudioInputPrice
	}
	if !dImageGenerationCallQuota.IsZero() {
		other["image_generation_call"] = true
		other["image_generation_call_price"] = imageGenerationCallPrice
	}
	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		ModelName:        logModel,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   int(useTimeSeconds),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
	})
}
