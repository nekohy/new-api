package model

import (
	"strings"
	"sync"
	"time"
)

// PricingGroupModel 模型归属：model -> 模型分组(pricing_groups.name)
type PricingGroupModel struct {
	ID             uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Model          string `json:"model" gorm:"type:varchar(255);not null;uniqueIndex"`
	ModelGroupName string `json:"model_group_name" gorm:"type:varchar(64);not null;index"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PricingGroupModel) TableName() string {
	return "pricing_group_models"
}

func EnsurePricingGroupModelTable() error {
	return DB.AutoMigrate(&PricingGroupModel{})
}

type pricingGroupModelCache struct {
	mu   sync.RWMutex
	data map[string]string // model -> modelGroup
}

var pgmCacheInstance = &pricingGroupModelCache{
	data: map[string]string{},
}

func InitPricingGroupModelCache() error {
	var rows []*PricingGroupModel
	if err := DB.Find(&rows).Error; err != nil {
		return err
	}
	tmp := make(map[string]string, len(rows))
	for _, r := range rows {
		if r == nil || r.Model == "" || r.ModelGroupName == "" {
			continue
		}
		tmp[r.Model] = r.ModelGroupName
	}
	pgmCacheInstance.mu.Lock()
	pgmCacheInstance.data = tmp
	pgmCacheInstance.mu.Unlock()
	return nil
}

func GetModelGroupForModel(modelName string) (string, bool) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return "", false
	}
	pgmCacheInstance.mu.RLock()
	defer pgmCacheInstance.mu.RUnlock()
	v, ok := pgmCacheInstance.data[modelName]
	return v, ok
}

func UpsertPricingGroupModel(modelName, modelGroupName string) error {
	modelName = strings.TrimSpace(modelName)
	modelGroupName = strings.TrimSpace(modelGroupName)
	if modelName == "" || modelGroupName == "" {
		return nil
	}
	now := time.Now().Unix()
	row := &PricingGroupModel{
		Model:          modelName,
		ModelGroupName: modelGroupName,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := DB.Save(row).Error; err != nil {
		return err
	}
	return InitPricingGroupModelCache()
}

func GetAllPricingGroupModels() ([]*PricingGroupModel, error) {
	var rows []*PricingGroupModel
	if err := DB.Order("model_group_name asc, model asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func DeletePricingGroupModel(modelName string) error {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil
	}
	if err := DB.Where("model = ?", modelName).Delete(&PricingGroupModel{}).Error; err != nil {
		return err
	}
	return InitPricingGroupModelCache()
}
