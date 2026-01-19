package helper

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// https://docs.claude.com/en/docs/build-with-claude/prompt-caching#1-hour-cache-duration
const claudeCacheCreation1hMultiplier = 6 / 3.75

// derefFloat64 安全解引用 float64 指针
func derefFloat64(ptr *float64) float64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

// derefInt 安全解引用 int 指针
func derefInt(ptr *int) int {
	if ptr == nil {
		return 0
	}
	return *ptr
}

// HandlePricingContext 处理定价上下文
func HandlePricingContext(ctx *gin.Context, relayInfo *relaycommon.RelayInfo) types.PricingContext {
	pricingContext := types.PricingContext{}

	autoGroup, exists := ctx.Get("auto_group")
	if exists {
		logger.LogDebug(ctx, fmt.Sprintf("final group: %s", autoGroup))
		relayInfo.UsingGroup = autoGroup.(string)
	}
	pricingContext.UsingGroup = relayInfo.UsingGroup

	return pricingContext
}

// ModelPriceHelper 获取模型价格信息
func ModelPriceHelper(c *gin.Context, info *relaycommon.RelayInfo, promptTokens int, meta *types.TokenCountMeta) (types.PriceData, error) {
	pricingContext := HandlePricingContext(c, info)

	spec, err := GetPriceSpec(info.UsingGroup, info.OriginModelName)
	if err != nil || spec == nil {
		if info.UserSetting.AcceptUnsetRatioModel {
			return types.PriceData{
				FreeModel: true,
				Context:   pricingContext,
			}, nil
		}
		return types.PriceData{}, fmt.Errorf("模型 %s 价格未配置，请联系管理员设置或开启自用模式；Model %s price not set, please contact admin or enable self-use mode", info.OriginModelName, info.OriginModelName)
	}

	var preConsumedQuota int
	var freeModel bool

	if spec.QuotaType == 1 {
		if meta.ImagePriceRatio != 0 {
			preConsumedQuota = int(spec.FixedPrice * meta.ImagePriceRatio * common.QuotaPerUnit)
		} else {
			preConsumedQuota = int(spec.FixedPrice * common.QuotaPerUnit)
		}
	} else {
		// 按量计费：预消费 = (promptTokens / 1M) * inputPrice * QuotaPerUnit
		preConsumedTokens := common.Max(promptTokens, common.PreConsumedQuota)
		if meta.MaxTokens != 0 {
			preConsumedTokens += meta.MaxTokens
		}
		preConsumedQuota = int(float64(preConsumedTokens) / 1000000 * spec.InputPrice * common.QuotaPerUnit)
	}

	if !operation_setting.GetQuotaSetting().EnableFreeModelPreConsume {
		if spec.QuotaType == 1 {
			if spec.FixedPrice == 0 {
				preConsumedQuota = 0
				freeModel = true
			}
		} else {
			if spec.InputPrice == 0 {
				preConsumedQuota = 0
				freeModel = true
			}
		}
	}

	priceData := types.PriceData{
		FreeModel: freeModel,
		Spec: types.PriceSpec{
			QuotaType:         spec.QuotaType,
			InputPrice:        spec.InputPrice,
			OutputPrice:       spec.OutputPrice,
			CacheReadPrice:    spec.CachePrice,
			CacheWritePrice:   spec.CacheCreationPrice,
			CacheWrite5mPrice: spec.CacheCreationPrice,
			CacheWrite1hPrice: spec.CacheCreationPrice * claudeCacheCreation1hMultiplier,
			ImagePrice:        spec.ImagePrice,
			AudioInputPrice:   spec.AudioInputPrice,
			AudioOutputPrice:  spec.AudioOutputPrice,
			FixedPrice:        spec.FixedPrice,
		},
		Context:           pricingContext,
		QuotaToPreConsume: preConsumedQuota,
	}

	if common.DebugEnabled {
		println(fmt.Sprintf("model_price_helper result: %s", priceData.ToSetting()))
	}
	info.PriceData = priceData
	return priceData, nil
}

// ModelPriceHelperPerCall 按次计费的 PriceHelper
func ModelPriceHelperPerCall(c *gin.Context, info *relaycommon.RelayInfo) types.PerCallPriceData {
	pricingContext := HandlePricingContext(c, info)

	spec, err := GetPriceSpec(info.UsingGroup, info.OriginModelName)
	var fixedPrice float64
	if err != nil || spec == nil {
		fixedPrice = 0.1
	} else {
		fixedPrice = spec.FixedPrice
		if fixedPrice == 0 {
			fixedPrice = spec.InputPrice / 1000
		}
	}

	quota := int(fixedPrice * common.QuotaPerUnit)
	priceData := types.PerCallPriceData{
		FixedPrice: fixedPrice,
		Quota:      quota,
		Context:    pricingContext,
	}
	return priceData
}

// ContainPrice 检查模型是否有价格配置
// 返回 true 表示至少存在一个有效的价格配置
func ContainPrice(modelName string) bool {
	if modelName == "" {
		return false
	}

	normalizedName := FormatMatchingModelName(modelName)

	var count int64
	err := model.DB.Model(&model.PricingGroupPrice{}).
		Where("model = ? OR model = ?", modelName, normalizedName).
		Where("(quota_type = 1 AND fixed_price IS NOT NULL) OR " +
			"(input_price IS NOT NULL OR output_price IS NOT NULL OR " +
			"cache_price IS NOT NULL OR cache_creation_price IS NOT NULL OR " +
			"image_price IS NOT NULL OR audio_input_price IS NOT NULL OR audio_output_price IS NOT NULL)").
		Count(&count).Error

	if err != nil {
		common.SysError("ContainPrice query failed: " + err.Error())
		return false
	}

	return count > 0
}

// FormatMatchingModelName 格式化模型名称用于匹配
func FormatMatchingModelName(modelName string) string {
	// 处理 gpt-4-gizmo-* 等特殊模型名
	if strings.HasPrefix(modelName, "gpt-4-gizmo-") {
		return "gpt-4-gizmo-*"
	}
	if strings.HasPrefix(modelName, "gpt-4o-gizmo-") {
		return "gpt-4o-gizmo-*"
	}
	// 处理 thinking-* 模型
	if strings.HasPrefix(modelName, "thinking-") {
		return "thinking-*"
	}
	return modelName
}

// PriceSpec 价格规格结构 ($/1M tokens)
type PriceSpec struct {
	Model              string
	Group              string
	InputPrice         float64
	OutputPrice        float64
	CachePrice         float64
	CacheCreationPrice float64
	ImagePrice         float64
	AudioInputPrice    float64
	AudioOutputPrice   float64
	FixedPrice         float64
	QuotaType          int
}

// isGroupPriceConfigured 检查分组价格是否已配置
func isGroupPriceConfigured(groupPrice *model.PricingGroupPrice) bool {
	if groupPrice == nil {
		return false
	}
	return groupPrice.InputPrice != nil ||
		groupPrice.OutputPrice != nil ||
		groupPrice.CachePrice != nil ||
		groupPrice.CacheCreationPrice != nil ||
		groupPrice.ImagePrice != nil ||
		groupPrice.AudioInputPrice != nil ||
		groupPrice.AudioOutputPrice != nil ||
		groupPrice.FixedPrice != nil ||
		groupPrice.QuotaType != nil
}

// GetPriceSpec 获取价格规格
func GetPriceSpec(group, modelName string) (*PriceSpec, error) {
	if modelName == "" {
		return nil, errors.New("missing model name")
	}

	normalizedName := FormatMatchingModelName(modelName)

	targetGroup := group
	if targetGroup == "" {
		targetGroup = "default"
	}

	// ========== 权限检查：用户分组是否有权访问该模型 ==========
	// 检查用户分组是否有权访问该模型（基于模型分组绑定）
	allowed, multiplier, err := model.CheckUserPricingAccess(targetGroup, modelName)
	if err != nil {
		// 模型未分配到模型分组，记录警告但继续处理（向后兼容）
		common.SysLog(fmt.Sprintf("Model access check warning for group=%s model=%s: %v", targetGroup, modelName, err))
	} else if !allowed {
		// 明确拒绝：用户分组未授权访问该模型所属的模型分组
		allowedGroups, _ := model.GetAllowedModelGroupsForUserGroup(targetGroup)
		return nil, fmt.Errorf("user group '%s' is not authorized to access this model (allowed model groups: %v)", targetGroup, allowedGroups)
	}
	// 如果 allowed=true，继续获取价格（multiplier 将在后续使用）
	_ = multiplier // TODO: 在计费时应用倍率
	// ========== 权限检查结束 ==========

	// 1. 尝试从分组价格缓存获取
	if groupPrice, found := model.GetPricingGroupPriceFromCache(targetGroup, modelName); found && groupPrice != nil {
		if isGroupPriceConfigured(groupPrice) {
			return &PriceSpec{
				Model:              modelName,
				Group:              targetGroup,
				InputPrice:         derefFloat64(groupPrice.InputPrice),
				OutputPrice:        derefFloat64(groupPrice.OutputPrice),
				CachePrice:         derefFloat64(groupPrice.CachePrice),
				CacheCreationPrice: derefFloat64(groupPrice.CacheCreationPrice),
				ImagePrice:         derefFloat64(groupPrice.ImagePrice),
				AudioInputPrice:    derefFloat64(groupPrice.AudioInputPrice),
				AudioOutputPrice:   derefFloat64(groupPrice.AudioOutputPrice),
				FixedPrice:         derefFloat64(groupPrice.FixedPrice),
				QuotaType:          derefInt(groupPrice.QuotaType),
			}, nil
		}
	}

	// 2. 尝试用格式化后的名称
	if normalizedName != modelName {
		if groupPrice, found := model.GetPricingGroupPriceFromCache(targetGroup, normalizedName); found && groupPrice != nil {
			if isGroupPriceConfigured(groupPrice) {
				return &PriceSpec{
					Model:              modelName,
					Group:              targetGroup,
					InputPrice:         derefFloat64(groupPrice.InputPrice),
					OutputPrice:        derefFloat64(groupPrice.OutputPrice),
					CachePrice:         derefFloat64(groupPrice.CachePrice),
					CacheCreationPrice: derefFloat64(groupPrice.CacheCreationPrice),
					ImagePrice:         derefFloat64(groupPrice.ImagePrice),
					AudioInputPrice:    derefFloat64(groupPrice.AudioInputPrice),
					AudioOutputPrice:   derefFloat64(groupPrice.AudioOutputPrice),
					FixedPrice:         derefFloat64(groupPrice.FixedPrice),
					QuotaType:          derefInt(groupPrice.QuotaType),
				}, nil
			}
		}
	}

	// 未找到价格配置（严格模式：不回退到 default 分组）
	return nil, fmt.Errorf("model %s price not configured in group %s", modelName, targetGroup)
}

// CalculateQuota 计算配额消耗
// 公式: quota = (tokens / 1M) * price * QuotaPerUnit
func CalculateQuota(spec *PriceSpec, usage *dto.Usage) int {
	if spec == nil {
		return 0
	}

	// 按次计费
	if spec.QuotaType == 1 {
		return int(math.Ceil(spec.FixedPrice * common.QuotaPerUnit))
	}

	// 按量计费
	if usage == nil {
		return 0
	}

	cost := 0.0
	cost += float64(usage.PromptTokens) / 1000000 * spec.InputPrice
	cost += float64(usage.CompletionTokens) / 1000000 * spec.OutputPrice

	// 缓存命中计费
	if usage.PromptCacheHitTokens > 0 {
		cost += float64(usage.PromptCacheHitTokens) / 1000000 * spec.CachePrice
	}

	// 缓存创建计费（Claude 5分钟缓存）
	if usage.ClaudeCacheCreation5mTokens > 0 {
		cost += float64(usage.ClaudeCacheCreation5mTokens) / 1000000 * spec.CacheCreationPrice
	}

	// 缓存创建计费（Claude 1小时缓存）
	if usage.ClaudeCacheCreation1hTokens > 0 {
		cost += float64(usage.ClaudeCacheCreation1hTokens) / 1000000 * spec.CacheCreationPrice * claudeCacheCreation1hMultiplier
	}

	if cost < 0 {
		return 0
	}
	return int(math.Ceil(cost * common.QuotaPerUnit))
}

// CalculateCost 计算成本分解
func CalculateCost(spec *PriceSpec, usage *dto.Usage) types.CostBreakdown {
	if spec == nil || usage == nil {
		return types.CostBreakdown{}
	}

	cost := types.CostBreakdown{}

	// 按次计费
	if spec.QuotaType == 1 {
		cost.FixedCost = spec.FixedPrice
		cost.TotalCost = spec.FixedPrice
		cost.Quota = int(math.Ceil(spec.FixedPrice * common.QuotaPerUnit))
		return cost
	}

	// 按量计费
	cost.TextInputCost = float64(usage.PromptTokens) / 1000000 * spec.InputPrice
	cost.TextOutputCost = float64(usage.CompletionTokens) / 1000000 * spec.OutputPrice

	// 缓存命中
	if usage.PromptCacheHitTokens > 0 {
		cost.CacheReadCost = float64(usage.PromptCacheHitTokens) / 1000000 * spec.CachePrice
	}

	// 缓存创建（5分钟）
	if usage.ClaudeCacheCreation5mTokens > 0 {
		cost.CacheWrite5mCost = float64(usage.ClaudeCacheCreation5mTokens) / 1000000 * spec.CacheCreationPrice
	}

	// 缓存创建（1小时）
	if usage.ClaudeCacheCreation1hTokens > 0 {
		cost.CacheWrite1hCost = float64(usage.ClaudeCacheCreation1hTokens) / 1000000 * spec.CacheCreationPrice * claudeCacheCreation1hMultiplier
	}

	// 音频
	if usage.PromptTokensDetails.AudioTokens > 0 {
		cost.AudioInputCost = float64(usage.PromptTokensDetails.AudioTokens) / 1000000 * spec.AudioInputPrice
	}
	if usage.CompletionTokenDetails.AudioTokens > 0 {
		cost.AudioOutputCost = float64(usage.CompletionTokenDetails.AudioTokens) / 1000000 * spec.AudioOutputPrice
	}

	// 总成本
	cost.TotalCost = cost.TextInputCost + cost.TextOutputCost +
		cost.CacheReadCost + cost.CacheWriteCost + cost.CacheWrite5mCost + cost.CacheWrite1hCost +
		cost.AudioInputCost + cost.AudioOutputCost + cost.ImageCost + cost.FixedCost

	if cost.TotalCost < 0 {
		cost.TotalCost = 0
	}
	cost.Quota = int(math.Ceil(cost.TotalCost * common.QuotaPerUnit))

	return cost
}
