package model

import (
	"strings"
	"sync"
	"time"

	"gorm.io/gorm/clause"
)

// PricingGroupPrice 定价分组模型价格 - 稀疏存储，只存分组内有覆盖配置的模型 ($/1M tokens)
// 价格字段语义：nil = 未设置（需要用户配置）, 0 = 免费, >0 = 具体价格
// QuotaType: 0=按量计费, 1=按次计费(固定价格)
type PricingGroupPrice struct {
	ID                 uint     `json:"id" gorm:"primaryKey;autoIncrement"`
	GroupName          string   `json:"group_name" gorm:"type:varchar(64);not null;uniqueIndex:idx_group_model"`
	Model              string   `json:"model" gorm:"type:varchar(255);not null;uniqueIndex:idx_group_model"`
	InputPrice         *float64 `json:"input_price" gorm:"type:decimal(12,6)"`
	OutputPrice        *float64 `json:"output_price" gorm:"type:decimal(12,6)"`
	CachePrice         *float64 `json:"cache_price" gorm:"type:decimal(12,6)"`
	CacheCreationPrice *float64 `json:"cache_creation_price" gorm:"type:decimal(12,6)"`
	ImagePrice         *float64 `json:"image_price" gorm:"type:decimal(12,6)"`
	AudioInputPrice    *float64 `json:"audio_input_price" gorm:"type:decimal(12,6)"`
	AudioOutputPrice   *float64 `json:"audio_output_price" gorm:"type:decimal(12,6)"`
	FixedPrice         *float64 `json:"fixed_price" gorm:"type:decimal(12,6)"`
	QuotaType          *int     `json:"quota_type" gorm:"type:int"`
	CreatedAt          int64    `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          int64    `json:"updated_at" gorm:"autoUpdateTime"`
}

// Float64Ptr 辅助函数：创建 float64 指针
func Float64Ptr(v float64) *float64 {
	return &v
}

// IntPtr 辅助函数：创建 int 指针
func IntPtr(v int) *int {
	return &v
}

// GetInputPriceValue 安全获取 InputPrice 值
func (g *PricingGroupPrice) GetInputPriceValue() (float64, bool) {
	if g.InputPrice == nil {
		return 0, false
	}
	return *g.InputPrice, true
}

// GetOutputPriceValue 安全获取 OutputPrice 值
func (g *PricingGroupPrice) GetOutputPriceValue() (float64, bool) {
	if g.OutputPrice == nil {
		return 0, false
	}
	return *g.OutputPrice, true
}

func (PricingGroupPrice) TableName() string {
	return "pricing_group_prices"
}

type groupModelPriceCacheKey struct {
	Group string
	Model string
}

var (
	groupModelPriceCache      = make(map[groupModelPriceCacheKey]*PricingGroupPrice)
	groupModelPriceCacheMutex sync.RWMutex
)

// InitPricingGroupPriceCache 初始化分组模型价格缓存
func InitPricingGroupPriceCache() error {
	var prices []*PricingGroupPrice
	if err := DB.Find(&prices).Error; err != nil {
		return err
	}
	tmp := make(map[groupModelPriceCacheKey]*PricingGroupPrice, len(prices))
	for _, p := range prices {
		if p != nil && p.GroupName != "" && p.Model != "" {
			key := groupModelPriceCacheKey{Group: p.GroupName, Model: p.Model}
			tmp[key] = p
		}
	}
	groupModelPriceCacheMutex.Lock()
	groupModelPriceCache = tmp
	groupModelPriceCacheMutex.Unlock()
	return nil
}

// GetPricingGroupPriceFromCache 从缓存获取分组模型价格
func GetPricingGroupPriceFromCache(group, model string) (*PricingGroupPrice, bool) {
	groupModelPriceCacheMutex.RLock()
	defer groupModelPriceCacheMutex.RUnlock()
	key := groupModelPriceCacheKey{Group: group, Model: model}
	p, ok := groupModelPriceCache[key]
	return p, ok
}

// GetGroupAllPrices 获取分组下所有模型价格
func GetGroupAllPrices(group string) ([]*PricingGroupPrice, error) {
	var prices []*PricingGroupPrice
	if err := DB.Where("group_name = ?", group).Order("model asc").Find(&prices).Error; err != nil {
		return nil, err
	}
	return prices, nil
}

// GetAllGroupNames 获取所有分组名称（从 group_model_prices 表中）
func GetAllGroupNames() ([]string, error) {
	var groups []string
	if err := DB.Model(&PricingGroupPrice{}).Distinct("group_name").Order("group_name asc").Pluck("group_name", &groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// UpsertPricingGroupPrice 创建或更新单个分组模型价格（使用 ON CONFLICT 原子操作）
func UpsertPricingGroupPrice(price *PricingGroupPrice) error {
	if price == nil || price.GroupName == "" || price.Model == "" {
		return nil
	}
	now := time.Now().Unix()
	price.UpdatedAt = now
	if price.CreatedAt == 0 {
		price.CreatedAt = now
	}

	// 使用 ON CONFLICT DO UPDATE 实现原子 upsert
	err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "group_name"}, {Name: "model"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"input_price", "output_price", "cache_price", "cache_creation_price",
			"image_price", "audio_input_price", "audio_output_price",
			"fixed_price", "quota_type", "updated_at",
		}),
	}).Create(price).Error

	if err != nil {
		return err
	}
	return InitPricingGroupPriceCache()
}

// BatchUpsertPricingGroupPrices 批量创建或更新分组模型价格（使用 ON CONFLICT 原子操作）
func BatchUpsertPricingGroupPrices(prices []*PricingGroupPrice) error {
	if len(prices) == 0 {
		return nil
	}
	now := time.Now().Unix()
	for _, p := range prices {
		if p != nil {
			p.UpdatedAt = now
			if p.CreatedAt == 0 {
				p.CreatedAt = now
			}
		}
	}

	// 过滤有效记录
	var validPrices []*PricingGroupPrice
	for _, price := range prices {
		if price != nil && price.GroupName != "" && price.Model != "" {
			validPrices = append(validPrices, price)
		}
	}

	if len(validPrices) == 0 {
		return nil
	}

	// 使用 ON CONFLICT DO UPDATE 批量 upsert（分批处理，每批 100 条）
	err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "group_name"}, {Name: "model"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"input_price", "output_price", "cache_price", "cache_creation_price",
			"image_price", "audio_input_price", "audio_output_price",
			"fixed_price", "quota_type", "updated_at",
		}),
	}).CreateInBatches(validPrices, 100).Error

	if err != nil {
		return err
	}
	return InitPricingGroupPriceCache()
}

// DeletePricingGroupPrice 删除单个分组模型价格
func DeletePricingGroupPrice(group, model string) error {
	if group == "" || model == "" {
		return nil
	}
	if err := DB.Where("group_name = ? AND model = ?", group, model).Delete(&PricingGroupPrice{}).Error; err != nil {
		return err
	}
	return InitPricingGroupPriceCache()
}

// BatchDeletePricingGroupPrices 批量删除分组模型价格
func BatchDeletePricingGroupPrices(group string, models []string) (int64, error) {
	if group == "" || len(models) == 0 {
		return 0, nil
	}
	result := DB.Where("group_name = ? AND model IN ?", group, models).Delete(&PricingGroupPrice{})
	if result.Error != nil {
		return 0, result.Error
	}
	if err := InitPricingGroupPriceCache(); err != nil {
		return result.RowsAffected, err
	}
	return result.RowsAffected, nil
}

// DeleteGroupAllPrices 删除整个分组的所有模型价格
func DeleteGroupAllPrices(group string) error {
	if group == "" {
		return nil
	}
	if err := DB.Where("group_name = ?", group).Delete(&PricingGroupPrice{}).Error; err != nil {
		return err
	}
	return InitPricingGroupPriceCache()
}

// GetPricingGroupPriceCacheCount 获取缓存中的分组模型价格数量
func GetPricingGroupPriceCacheCount() int {
	groupModelPriceCacheMutex.RLock()
	defer groupModelPriceCacheMutex.RUnlock()
	return len(groupModelPriceCache)
}

// GetGroupConfiguredModels 获取分组内已配置的模型列表
func GetGroupConfiguredModels(group string) map[string]bool {
	groupModelPriceCacheMutex.RLock()
	defer groupModelPriceCacheMutex.RUnlock()
	result := make(map[string]bool)
	for key := range groupModelPriceCache {
		if key.Group == group {
			result[key.Model] = true
		}
	}
	return result
}

// SyncModelsToPricingGroupPrices 从 Channel 同步模型到 pricing_group_prices 表
// modelGroups: map[model][]groups - 模型与分组的映射关系
// 只创建不存在的分组模型记录（价格字段全为 nil），不覆盖已存在的
func SyncModelsToPricingGroupPrices(modelGroups map[string][]string) error {
	if len(modelGroups) == 0 {
		return nil
	}

	// 获取已存在的分组模型
	groupModelPriceCacheMutex.RLock()
	existingPrices := make(map[groupModelPriceCacheKey]bool, len(groupModelPriceCache))
	for key := range groupModelPriceCache {
		existingPrices[key] = true
	}
	groupModelPriceCacheMutex.RUnlock()

	// 收集需要创建的新记录
	var newPrices []*PricingGroupPrice
	now := time.Now().Unix()

	for model, groups := range modelGroups {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		for _, group := range groups {
			group = strings.TrimSpace(group)
			if group == "" {
				continue
			}
			key := groupModelPriceCacheKey{Group: group, Model: model}
			if !existingPrices[key] {
				// 创建新记录，所有价格字段为 nil（未设置）
				newPrices = append(newPrices, &PricingGroupPrice{
					GroupName: group,
					Model:     model,
					// 所有价格字段保持 nil
					CreatedAt: now,
					UpdatedAt: now,
				})
			}
		}
	}

	if len(newPrices) == 0 {
		return nil
	}

	// 批量插入，使用 DoNothing 避免覆盖已存在记录
	if err := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "group_name"}, {Name: "model"}},
		DoNothing: true,
	}).CreateInBatches(&newPrices, 100).Error; err != nil {
		return err
	}

	return InitPricingGroupPriceCache()
}

// CleanupOrphanedPricingGroupPrices 清理孤立的定价分组价格记录
// 删除 pricing_group_prices 中存在但 abilities 表中不存在的 (group, model) 组合
func CleanupOrphanedPricingGroupPrices() error {
	// 从 abilities 表获取所有有效的 (group, model) 组合
	var validPairs []struct {
		Group string `gorm:"column:group"`
		Model string `gorm:"column:model"`
	}
	if err := DB.Model(&Ability{}).
		Select("DISTINCT `group`, model").
		Find(&validPairs).Error; err != nil {
		return err
	}

	// 构建有效组合的 map
	validSet := make(map[groupModelPriceCacheKey]bool, len(validPairs))
	for _, p := range validPairs {
		if p.Group != "" && p.Model != "" {
			validSet[groupModelPriceCacheKey{Group: p.Group, Model: p.Model}] = true
		}
	}

	// 获取 group_model_prices 中所有记录
	var allPrices []*PricingGroupPrice
	if err := DB.Find(&allPrices).Error; err != nil {
		return err
	}

	// 找出需要删除的记录
	var toDelete []uint
	for _, p := range allPrices {
		key := groupModelPriceCacheKey{Group: p.GroupName, Model: p.Model}
		if !validSet[key] {
			toDelete = append(toDelete, p.ID)
		}
	}

	if len(toDelete) == 0 {
		return nil
	}

	// 批量删除
	if err := DB.Where("id IN ?", toDelete).Delete(&PricingGroupPrice{}).Error; err != nil {
		return err
	}

	return InitPricingGroupPriceCache()
}

// SyncModelsToGroupPrices 同步模型到分组定价表（别名）
// 兼容旧调用名称
func SyncModelsToGroupPrices(modelGroups map[string][]string) error {
	return SyncModelsToPricingGroupPrices(modelGroups)
}

// CleanupOrphanedGroupModelPrices 清理孤立的分组模型价格记录（别名）
// 兼容旧调用名称
func CleanupOrphanedGroupModelPrices() error {
	return CleanupOrphanedPricingGroupPrices()
}
