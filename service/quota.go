package service

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/gin-gonic/gin"
)

// hasCustomPrice 检查模型是否有自定义价格配置
func hasCustomPrice(modelName string) bool {
	return helper.ContainPrice(modelName)
}

// calculateAudioQuotaByPrice 使用 Price 计算音频相关配额
func calculateAudioQuotaByPrice(spec *helper.PriceSpec, textInputTokens, textOutputTokens, audioInputTokens, audioOutputTokens int) int {
	if spec == nil {
		return 0
	}

	// 按次计费
	if spec.QuotaType == 1 {
		return int(math.Ceil(spec.FixedPrice * common.QuotaPerUnit))
	}

	// 按量计费：cost = (tokens / 1M) * price
	cost := 0.0
	cost += float64(textInputTokens) / 1000000 * spec.InputPrice
	cost += float64(textOutputTokens) / 1000000 * spec.OutputPrice
	cost += float64(audioInputTokens) / 1000000 * spec.AudioInputPrice
	cost += float64(audioOutputTokens) / 1000000 * spec.AudioOutputPrice

	if cost <= 0 && spec.InputPrice > 0 {
		return 1 // 最小计费
	}
	return int(math.Ceil(cost * common.QuotaPerUnit))
}

func PreWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.RealtimeUsage) error {
	if relayInfo.PriceData.Spec.QuotaType == 1 {
		return nil
	}
	userQuota, err := model.GetUserQuota(relayInfo.UserId, false)
	if err != nil {
		return err
	}

	token, err := model.GetTokenByKey(strings.TrimPrefix(relayInfo.TokenKey, "sk-"), false)
	if err != nil {
		return err
	}

	modelName := relayInfo.OriginModelName

	// 处理 auto_group
	autoGroup, exists := common.GetContextKey(ctx, constant.ContextKeyAutoGroup)
	if exists {
		log.Printf("final group: %s", autoGroup.(string))
		relayInfo.UsingGroup = autoGroup.(string)
	}

	spec, _ := helper.GetPriceSpec(relayInfo.UsingGroup, modelName)
	quota := calculateAudioQuotaByPrice(
		spec,
		usage.InputTokenDetails.TextTokens,
		usage.OutputTokenDetails.TextTokens,
		usage.InputTokenDetails.AudioTokens,
		usage.OutputTokenDetails.AudioTokens,
	)

	if userQuota < quota {
		return fmt.Errorf("user quota is not enough, user quota: %s, need quota: %s", logger.FormatQuota(userQuota), logger.FormatQuota(quota))
	}

	if !token.UnlimitedQuota && token.RemainQuota < quota {
		return fmt.Errorf("token quota is not enough, token remain quota: %s, need quota: %s", logger.FormatQuota(token.RemainQuota), logger.FormatQuota(quota))
	}

	err = PostConsumeQuota(relayInfo, quota, 0, false)
	if err != nil {
		return err
	}
	logger.LogInfo(ctx, "realtime streaming consume quota success, quota: "+fmt.Sprintf("%d", quota))
	return nil
}

func PostWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelName string,
	usage *dto.RealtimeUsage, extraContent string) {

	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	tokenName := ctx.GetString("token_name")

	spec, _ := helper.GetPriceSpec(relayInfo.UsingGroup, modelName)
	quota := calculateAudioQuotaByPrice(
		spec,
		usage.InputTokenDetails.TextTokens,
		usage.OutputTokenDetails.TextTokens,
		usage.InputTokenDetails.AudioTokens,
		usage.OutputTokenDetails.AudioTokens,
	)

	totalTokens := usage.TotalTokens
	var logContent string
	if spec != nil && spec.QuotaType == 1 {
		logContent = fmt.Sprintf("固定价格 $%.4f/次", spec.FixedPrice)
	} else if spec != nil {
		logContent = fmt.Sprintf("输入价格 $%.4f/1M, 输出价格 $%.4f/1M, 音频输入 $%.4f/1M, 音频输出 $%.4f/1M",
			spec.InputPrice, spec.OutputPrice, spec.AudioInputPrice, spec.AudioOutputPrice)
	}

	if totalTokens == 0 {
		quota = 0
		logContent += "（可能是上游超时）"
		logger.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s, pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, modelName, relayInfo.FinalPreConsumedQuota))
	} else {
		model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, quota)
	}

	if extraContent != "" {
		logContent += ", " + extraContent
	}

	other := GenerateWssOtherInfoByPrice(ctx, relayInfo, usage, spec)
	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		ModelName:        modelName,
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

func PostClaudeConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage) {
	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	promptTokens := usage.PromptTokens
	completionTokens := usage.CompletionTokens
	modelName := relayInfo.OriginModelName
	tokenName := ctx.GetString("token_name")

	spec, _ := helper.GetPriceSpec(relayInfo.UsingGroup, modelName)

	cacheTokens := usage.PromptTokensDetails.CachedTokens
	cacheCreationTokens := usage.PromptTokensDetails.CachedCreationTokens
	cacheCreationTokens5m := usage.ClaudeCacheCreation5mTokens
	cacheCreationTokens1h := usage.ClaudeCacheCreation1hTokens

	// OpenRouter 特殊处理
	if relayInfo.ChannelType == constant.ChannelTypeOpenRouter {
		promptTokens -= cacheTokens
		promptTokens -= cacheCreationTokens
	}

	// 计算配额
	var quota int
	if spec == nil {
		quota = 0
	} else if spec.QuotaType == 1 {
		// 按次计费
		quota = int(math.Ceil(spec.FixedPrice * common.QuotaPerUnit))
	} else {
		// 按量计费
		cost := 0.0
		cost += float64(promptTokens) / 1000000 * spec.InputPrice
		cost += float64(completionTokens) / 1000000 * spec.OutputPrice
		cost += float64(cacheTokens) / 1000000 * spec.CachePrice
		cost += float64(cacheCreationTokens5m) / 1000000 * spec.CacheCreationPrice
		cost += float64(cacheCreationTokens1h) / 1000000 * spec.CacheCreationPrice * (6 / 3.75) // 1h multiplier
		remainingCacheCreationTokens := cacheCreationTokens - cacheCreationTokens5m - cacheCreationTokens1h
		if remainingCacheCreationTokens > 0 {
			cost += float64(remainingCacheCreationTokens) / 1000000 * spec.CacheCreationPrice
		}
		if cost <= 0 && spec.InputPrice > 0 {
			quota = 1
		} else {
			quota = int(math.Ceil(cost * common.QuotaPerUnit))
		}
	}

	totalTokens := promptTokens + completionTokens
	var logContent string

	if totalTokens == 0 {
		quota = 0
		logContent += "（可能是上游出错）"
		logger.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s, pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, modelName, relayInfo.FinalPreConsumedQuota))
	} else {
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
		err := PostConsumeQuota(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota, true)
		if err != nil {
			logger.LogError(ctx, "error consuming token remain quota: "+err.Error())
		}
	}

	other := GenerateClaudeOtherInfoByPrice(ctx, relayInfo, spec, cacheTokens, cacheCreationTokens, cacheCreationTokens5m, cacheCreationTokens1h)
	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		ModelName:        modelName,
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

func PostAudioConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, extraContent string) {
	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	tokenName := ctx.GetString("token_name")

	spec, _ := helper.GetPriceSpec(relayInfo.UsingGroup, relayInfo.OriginModelName)
	quota := calculateAudioQuotaByPrice(
		spec,
		usage.PromptTokensDetails.TextTokens,
		usage.CompletionTokenDetails.TextTokens,
		usage.PromptTokensDetails.AudioTokens,
		usage.CompletionTokenDetails.AudioTokens,
	)

	totalTokens := usage.TotalTokens
	var logContent string
	if spec != nil && spec.QuotaType == 1 {
		logContent = fmt.Sprintf("固定价格 $%.4f/次", spec.FixedPrice)
	} else if spec != nil {
		logContent = fmt.Sprintf("输入价格 $%.4f/1M, 输出价格 $%.4f/1M, 音频输入 $%.4f/1M, 音频输出 $%.4f/1M",
			spec.InputPrice, spec.OutputPrice, spec.AudioInputPrice, spec.AudioOutputPrice)
	}

	if totalTokens == 0 {
		quota = 0
		logContent += "（可能是上游超时）"
		logger.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s, pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, relayInfo.OriginModelName, relayInfo.FinalPreConsumedQuota))
	} else {
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
		err := PostConsumeQuota(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota, true)
		if err != nil {
			logger.LogError(ctx, "error consuming token remain quota: "+err.Error())
		}
	}

	if extraContent != "" {
		logContent += ", " + extraContent
	}

	other := GenerateAudioOtherInfoByPrice(ctx, relayInfo, usage, spec)
	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ModelName:        relayInfo.OriginModelName,
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

func PreConsumeTokenQuota(relayInfo *relaycommon.RelayInfo, quota int) error {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if relayInfo.IsPlayground {
		return nil
	}
	token, err := model.GetTokenByKey(relayInfo.TokenKey, false)
	if err != nil {
		return err
	}
	if !relayInfo.TokenUnlimited && token.RemainQuota < quota {
		return fmt.Errorf("token quota is not enough, token remain quota: %s, need quota: %s", logger.FormatQuota(token.RemainQuota), logger.FormatQuota(quota))
	}
	err = model.DecreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
	if err != nil {
		return err
	}
	return nil
}

func PostConsumeQuota(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int, sendEmail bool) (err error) {
	if quota > 0 {
		err = model.DecreaseUserQuota(relayInfo.UserId, quota)
	} else {
		err = model.IncreaseUserQuota(relayInfo.UserId, -quota, false)
	}
	if err != nil {
		return err
	}

	if !relayInfo.IsPlayground {
		if quota > 0 {
			err = model.DecreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		} else {
			err = model.IncreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, -quota)
		}
		if err != nil {
			return err
		}
	}

	if sendEmail {
		if (quota + preConsumedQuota) != 0 {
			checkAndSendQuotaNotify(relayInfo, quota, preConsumedQuota)
		}
	}

	return nil
}

func checkAndSendQuotaNotify(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int) {
	gopool.Go(func() {
		userSetting := relayInfo.UserSetting
		threshold := common.QuotaRemindThreshold
		if userSetting.QuotaWarningThreshold != 0 {
			threshold = int(userSetting.QuotaWarningThreshold)
		}

		quotaTooLow := false
		consumeQuota := quota + preConsumedQuota
		if relayInfo.UserQuota-consumeQuota < threshold {
			quotaTooLow = true
		}
		if quotaTooLow {
			prompt := "您的额度即将用尽"
			topUpLink := fmt.Sprintf("%s/console/topup", system_setting.ServerAddress)

			var content string
			var values []interface{}

			notifyType := userSetting.NotifyType
			if notifyType == "" {
				notifyType = dto.NotifyTypeEmail
			}

			if notifyType == dto.NotifyTypeBark {
				content = "{{value}}，剩余额度：{{value}}，请及时充值"
				values = []interface{}{prompt, logger.FormatQuota(relayInfo.UserQuota)}
			} else if notifyType == dto.NotifyTypeGotify {
				content = "{{value}}，当前剩余额度为 {{value}}，请及时充值。"
				values = []interface{}{prompt, logger.FormatQuota(relayInfo.UserQuota)}
			} else {
				content = "{{value}}，当前剩余额度为 {{value}}，为了不影响您的使用，请及时充值。<br/>充值链接：<a href='{{value}}'>{{value}}</a>"
				values = []interface{}{prompt, logger.FormatQuota(relayInfo.UserQuota), topUpLink, topUpLink}
			}

			err := NotifyUser(relayInfo.UserId, relayInfo.UserEmail, relayInfo.UserSetting, dto.NewNotify(dto.NotifyTypeQuotaExceed, prompt, content, values))
			if err != nil {
				common.SysError(fmt.Sprintf("failed to send quota notify to user %d: %s", relayInfo.UserId, err.Error()))
			}
		}
	})
}
