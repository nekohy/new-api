package controller

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// PricingGroupPriceRequest 分组模型价格请求结构 ($/1M tokens)
type PricingGroupPriceRequest struct {
	Model              string   `json:"model"`
	SourceGroup        string   `json:"source_group,omitempty"` // 源分组（用于获取默认值或倍率计算）
	PriceMultiplier    *float64 `json:"price_multiplier,omitempty"`
	InputPrice         *float64 `json:"input_price,omitempty"`
	OutputPrice        *float64 `json:"output_price,omitempty"`
	CachePrice         *float64 `json:"cache_price,omitempty"`
	CacheCreationPrice *float64 `json:"cache_creation_price,omitempty"`
	ImagePrice         *float64 `json:"image_price,omitempty"`
	AudioInputPrice    *float64 `json:"audio_input_price,omitempty"`
	AudioOutputPrice   *float64 `json:"audio_output_price,omitempty"`
	FixedPrice         *float64 `json:"fixed_price,omitempty"`
	QuotaType          *int     `json:"quota_type,omitempty"`
}

// SyncFromGroupRequest 从分组同步价格请求
type SyncFromGroupRequest struct {
	SourceGroup string  `json:"source_group"` // 源分组名称
	Multiplier  float64 `json:"multiplier"`   // 必填，1 = 原价，1.5 = 1.5倍
	Mode        string  `json:"mode"`         // "all" | "missing"
}

// ApplyMultiplierRequest 应用倍率请求
type ApplyMultiplierRequest struct {
	SourceGroup string   `json:"source_group"` // 源分组（可选，默认从当前分组现有价格计算）
	Models      []string `json:"models"`       // 空=全组
	Multiplier  float64  `json:"multiplier"`   // 倍率
}

// GetGroupPrices 获取分组下所有模型价格
// GET /api/pricing-groups/:group/prices
func GetGroupPrices(c *gin.Context) {
	groupName := c.Param("group")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少分组名称",
		})
		return
	}

	// URL 解码
	decodedName, err := url.PathUnescape(groupName)
	if err != nil {
		decodedName = groupName
	}

	prices, err := model.GetGroupAllPrices(decodedName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    prices,
	})
}

// UpdatePricingGroupPriceByName 更新分组内单个模型价格
// PUT /api/pricing-groups/:group/prices/:model
func UpdatePricingGroupPriceByName(c *gin.Context) {
	groupName := c.Param("group")
	modelName := c.Param("model")
	if groupName == "" || modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少分组或模型名称",
		})
		return
	}

	// URL 解码
	decodedGroupName, _ := url.PathUnescape(groupName)
	decodedModelName, _ := url.PathUnescape(modelName)

	var req PricingGroupPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	now := time.Now().Unix()
	price := &model.PricingGroupPrice{
		GroupName:          decodedGroupName,
		Model:              decodedModelName,
		InputPrice:         req.InputPrice, // 直接使用指针，nil 表示未设置
		OutputPrice:        req.OutputPrice,
		CachePrice:         req.CachePrice,
		CacheCreationPrice: req.CacheCreationPrice,
		ImagePrice:         req.ImagePrice,
		AudioInputPrice:    req.AudioInputPrice,
		AudioOutputPrice:   req.AudioOutputPrice,
		FixedPrice:         req.FixedPrice,
		QuotaType:          req.QuotaType,
		UpdatedAt:          now,
	}

	if err := model.UpsertPricingGroupPrice(price); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "更新成功",
	})
}

// DeletePricingGroupPriceByName 删除分组内单个模型价格
// DELETE /api/pricing-groups/:group/prices/:model
func DeletePricingGroupPriceByName(c *gin.Context) {
	groupName := c.Param("group")
	modelName := c.Param("model")
	if groupName == "" || modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少分组或模型名称",
		})
		return
	}

	// URL 解码
	decodedGroupName, _ := url.PathUnescape(groupName)
	decodedModelName, _ := url.PathUnescape(modelName)

	if err := model.DeletePricingGroupPrice(decodedGroupName, decodedModelName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// BatchDeletePricingGroupPrices 批量删除分组内模型价格
// POST /api/pricing-groups/:group/prices/batch-delete
func BatchDeletePricingGroupPrices(c *gin.Context) {
	groupName := c.Param("group")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少分组名称",
		})
		return
	}

	// URL 解码
	decodedGroupName, _ := url.PathUnescape(groupName)

	var req struct {
		Models []string `json:"models" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	if len(req.Models) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "模型列表不能为空",
		})
		return
	}

	count, err := model.BatchDeletePricingGroupPrices(decodedGroupName, req.Models)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "批量删除成功",
		"count":   count,
	})
}

// AddModelToGroup 添加模型到分组
// POST /api/pricing-groups/:group/prices
func AddModelToGroup(c *gin.Context) {
	groupName := c.Param("group")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少分组名称",
		})
		return
	}

	// URL 解码
	decodedGroupName, err := url.PathUnescape(groupName)
	if err != nil {
		decodedGroupName = groupName
	}

	var req PricingGroupPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少模型名称",
		})
		return
	}

	now := time.Now().Unix()
	groupPrice := &model.PricingGroupPrice{
		GroupName: decodedGroupName,
		Model:     req.Model,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 尝试获取源分组价格作为默认值（如果指定了 PriceMultiplier 或需要默认值）
	var sourcePrice *model.PricingGroupPrice
	if req.SourceGroup != "" {
		sourcePrice, _ = model.GetPricingGroupPriceFromCache(req.SourceGroup, req.Model)
	}

	if req.PriceMultiplier != nil && sourcePrice != nil {
		// 价格乘数模式（基于源分组）
		applyMultiplierToPrice(groupPrice, sourcePrice, *req.PriceMultiplier)
	} else {
		// 直接设置模式（传入的指针直接使用）
		groupPrice.InputPrice = req.InputPrice
		groupPrice.OutputPrice = req.OutputPrice
		groupPrice.CachePrice = req.CachePrice
		groupPrice.CacheCreationPrice = req.CacheCreationPrice
		groupPrice.ImagePrice = req.ImagePrice
		groupPrice.AudioInputPrice = req.AudioInputPrice
		groupPrice.AudioOutputPrice = req.AudioOutputPrice
		groupPrice.FixedPrice = req.FixedPrice
		groupPrice.QuotaType = req.QuotaType
	}

	if err := model.UpsertPricingGroupPrice(groupPrice); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "模型添加成功",
	})
}

// BatchAddModelsToGroup 批量添加模型到分组
// POST /api/pricing-groups/:group/prices/batch
func BatchAddModelsToGroup(c *gin.Context) {
	groupName := c.Param("group")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少分组名称",
		})
		return
	}

	// URL 解码
	decodedGroupName, err := url.PathUnescape(groupName)
	if err != nil {
		decodedGroupName = groupName
	}

	var req []PricingGroupPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	now := time.Now().Unix()
	var prices []*model.PricingGroupPrice

	for _, r := range req {
		if r.Model == "" {
			continue
		}

		groupPrice := &model.PricingGroupPrice{
			GroupName: decodedGroupName,
			Model:     r.Model,
			CreatedAt: now,
			UpdatedAt: now,
		}

		// 尝试获取源分组价格（如果指定了）
		var sourcePrice *model.PricingGroupPrice
		if r.SourceGroup != "" {
			sourcePrice, _ = model.GetPricingGroupPriceFromCache(r.SourceGroup, r.Model)
		}

		if r.PriceMultiplier != nil && sourcePrice != nil {
			// 价格乘数模式（基于源分组）
			applyMultiplierToPrice(groupPrice, sourcePrice, *r.PriceMultiplier)
		} else {
			// 直接设置模式（传入的指针直接使用）
			groupPrice.InputPrice = r.InputPrice
			groupPrice.OutputPrice = r.OutputPrice
			groupPrice.CachePrice = r.CachePrice
			groupPrice.CacheCreationPrice = r.CacheCreationPrice
			groupPrice.ImagePrice = r.ImagePrice
			groupPrice.AudioInputPrice = r.AudioInputPrice
			groupPrice.AudioOutputPrice = r.AudioOutputPrice
			groupPrice.FixedPrice = r.FixedPrice
			groupPrice.QuotaType = r.QuotaType
		}

		prices = append(prices, groupPrice)
	}

	if err := model.BatchUpsertPricingGroupPrices(prices); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "批量添加成功",
		"count":   len(prices),
	})
}

// SyncFromGroup 从源分组同步价格到目标分组
// POST /api/pricing-groups/:group/prices/sync-from-group
func SyncFromGroup(c *gin.Context) {
	targetGroup := c.Param("group")
	if targetGroup == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少目标分组名称",
		})
		return
	}

	// URL 解码
	decodedTargetGroup, err := url.PathUnescape(targetGroup)
	if err != nil {
		decodedTargetGroup = targetGroup
	}

	var req SyncFromGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	if req.SourceGroup == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少源分组名称",
		})
		return
	}

	decodedSourceGroup := strings.TrimSpace(req.SourceGroup)
	if decodedSourceGroup == decodedTargetGroup {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "源分组和目标分组不能相同",
		})
		return
	}

	if req.Multiplier <= 0 {
		req.Multiplier = 1
	}

	// 1. 获取源分组的所有模型价格
	sourcePrices, err := model.GetGroupAllPrices(decodedSourceGroup)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取源分组价格失败: " + err.Error(),
		})
		return
	}
	if len(sourcePrices) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"count":   0,
			"message": "源分组没有模型价格可同步",
		})
		return
	}

	// 2. 如果是 "missing" 模式，过滤掉目标分组已配置的
	existingModels := model.GetGroupConfiguredModels(decodedTargetGroup)
	var toSync []*model.PricingGroupPrice
	for _, src := range sourcePrices {
		if req.Mode == "missing" && existingModels[src.Model] {
			continue
		}
		toSync = append(toSync, src)
	}

	if len(toSync) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"count":   0,
			"message": "没有需要同步的模型",
		})
		return
	}

	// 3. 批量生成目标分组价格 = 源价格 × 倍率
	now := time.Now().Unix()
	var targetPrices []*model.PricingGroupPrice
	for _, src := range toSync {
		targetPrice := &model.PricingGroupPrice{
			GroupName: decodedTargetGroup,
			Model:     src.Model,
			CreatedAt: now,
			UpdatedAt: now,
		}
		// 应用倍率到所有价格字段
		applyMultiplierToPrice(targetPrice, src, req.Multiplier)
		targetPrices = append(targetPrices, targetPrice)
	}

	// 4. 批量 Upsert
	if err := model.BatchUpsertPricingGroupPrices(targetPrices); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   len(targetPrices),
		"message": "同步成功",
	})
}

// ApplyMultiplier 对选中模型应用倍率（基于源分组价格）
// POST /api/pricing-groups/:group/prices/apply-multiplier
func ApplyMultiplier(c *gin.Context) {
	groupName := c.Param("group")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少分组名称",
		})
		return
	}

	// URL 解码
	decodedGroupName, err := url.PathUnescape(groupName)
	if err != nil {
		decodedGroupName = groupName
	}

	var req ApplyMultiplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	if req.Multiplier <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "倍率必须大于0",
		})
		return
	}

	// 确定源分组（默认使用当前分组）
	sourceGroup := decodedGroupName
	if req.SourceGroup != "" {
		sourceGroup = strings.TrimSpace(req.SourceGroup)
	}

	// 确定要处理的模型列表
	var modelsToProcess []string
	if len(req.Models) > 0 {
		modelsToProcess = req.Models
	} else {
		// 获取源分组内所有已配置的模型
		existingModels := model.GetGroupConfiguredModels(sourceGroup)
		for m := range existingModels {
			modelsToProcess = append(modelsToProcess, m)
		}
	}

	if len(modelsToProcess) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"count":   0,
			"message": "没有需要处理的模型",
		})
		return
	}

	// 生成新价格（基于源分组价格 × 倍率）
	now := time.Now().Unix()
	var groupPrices []*model.PricingGroupPrice
	for _, modelName := range modelsToProcess {
		srcPrice, ok := model.GetPricingGroupPriceFromCache(sourceGroup, modelName)
		if !ok {
			continue
		}
		targetPrice := &model.PricingGroupPrice{
			GroupName: decodedGroupName,
			Model:     modelName,
			CreatedAt: now,
			UpdatedAt: now,
		}
		// 应用倍率到所有价格字段
		applyMultiplierToPrice(targetPrice, srcPrice, req.Multiplier)
		groupPrices = append(groupPrices, targetPrice)
	}

	if len(groupPrices) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"count":   0,
			"message": "没有找到对应的分组价格",
		})
		return
	}

	// 批量 Upsert
	if err := model.BatchUpsertPricingGroupPrices(groupPrices); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   len(groupPrices),
		"message": "应用倍率成功",
	})
}

// applyMultiplierToPrice 将源价格乘以倍率后设置到目标价格
func applyMultiplierToPrice(dst, src *model.PricingGroupPrice, m float64) {
	if src.InputPrice != nil {
		dst.InputPrice = model.Float64Ptr(*src.InputPrice * m)
	}
	if src.OutputPrice != nil {
		dst.OutputPrice = model.Float64Ptr(*src.OutputPrice * m)
	}
	if src.CachePrice != nil {
		dst.CachePrice = model.Float64Ptr(*src.CachePrice * m)
	}
	if src.CacheCreationPrice != nil {
		dst.CacheCreationPrice = model.Float64Ptr(*src.CacheCreationPrice * m)
	}
	if src.ImagePrice != nil {
		dst.ImagePrice = model.Float64Ptr(*src.ImagePrice * m)
	}
	if src.AudioInputPrice != nil {
		dst.AudioInputPrice = model.Float64Ptr(*src.AudioInputPrice * m)
	}
	if src.AudioOutputPrice != nil {
		dst.AudioOutputPrice = model.Float64Ptr(*src.AudioOutputPrice * m)
	}
	if src.FixedPrice != nil {
		dst.FixedPrice = model.Float64Ptr(*src.FixedPrice * m)
	}
	dst.QuotaType = src.QuotaType
}

// getFloatOrDefault 返回指针值，若为 nil 则返回默认值
func getFloatOrDefault(ptr *float64, defaultVal float64) float64 {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}

// getIntOrDefault 返回指针值，若为 nil 则返回默认值
func getIntOrDefault(ptr *int, defaultVal int) int {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}

// getFloatPtrOrDefault 返回指针，若传入为 nil 则使用默认值
func getFloatPtrOrDefault(ptr *float64, defaultVal float64) *float64 {
	if ptr != nil {
		return ptr
	}
	return model.Float64Ptr(defaultVal)
}

// getIntPtrOrDefault 返回指针，若传入为 nil 则使用默认值
func getIntPtrOrDefault(ptr *int, defaultVal int) *int {
	if ptr != nil {
		return ptr
	}
	return model.IntPtr(defaultVal)
}
