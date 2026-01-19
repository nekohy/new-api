package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

var ErrInvalidPricingGroupName = errors.New("invalid pricing group name")

// PricingGroup 定价分组元数据表
type PricingGroup struct {
	ID          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(64);not null;uniqueIndex"`
	Description string `json:"description" gorm:"type:varchar(255);not null;default:''"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PricingGroup) TableName() string {
	return "pricing_groups"
}

// CreatePricingGroup 创建新分组
func CreatePricingGroup(name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrInvalidPricingGroupName
	}
	now := time.Now().Unix()
	row := &PricingGroup{
		Name:        name,
		Description: strings.TrimSpace(description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoNothing: true,
	}).Create(row).Error
}

// GetAllPricingGroupNames 获取所有分组名称列表
func GetAllPricingGroupNames() ([]string, error) {
	var names []string
	if err := DB.Model(&PricingGroup{}).Order("name asc").Pluck("name", &names).Error; err != nil {
		return nil, err
	}
	return names, nil
}

// GetPricingGroupDescription 获取分组描述
func GetPricingGroupDescription(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalidPricingGroupName
	}
	var desc string
	if err := DB.Model(&PricingGroup{}).Where("name = ?", name).Pluck("description", &desc).Error; err != nil {
		return "", err
	}
	return desc, nil
}

// GetPricingGroupDescriptions 批量获取分组描述（消除 N+1 查询）
// 返回 map[分组名]描述，若分组不存在则 map 中无对应 key
func GetPricingGroupDescriptions(names []string) (map[string]string, error) {
	if len(names) == 0 {
		return make(map[string]string), nil
	}

	// 去重并过滤空名称
	uniqueNames := make([]string, 0, len(names))
	seen := make(map[string]bool)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" && !seen[name] {
			uniqueNames = append(uniqueNames, name)
			seen[name] = true
		}
	}

	if len(uniqueNames) == 0 {
		return make(map[string]string), nil
	}

	var groups []struct {
		Name        string `gorm:"column:name"`
		Description string `gorm:"column:description"`
	}

	if err := DB.Table("pricing_groups").
		Select("name, description").
		Where("name IN ?", uniqueNames).
		Find(&groups).Error; err != nil {
		return nil, err
	}

	result := make(map[string]string, len(groups))
	for _, g := range groups {
		result[g.Name] = g.Description
	}
	return result, nil
}

// EnsurePricingGroupTable 确保 pricing_groups 表存在（用于 SQLite 跳过 AutoMigrate 的情况）
func EnsurePricingGroupTable() error {
	// 检查表是否已存在
	if DB.Migrator().HasTable(&PricingGroup{}) {
		return nil
	}
	return DB.AutoMigrate(&PricingGroup{})
}

// BackfillPricingGroups 从 group_model_prices 抽取分组，回填 pricing_groups（幂等）
func BackfillPricingGroups() error {
	if err := EnsurePricingGroupTable(); err != nil {
		return err
	}
	var names []string
	if err := DB.Model(&PricingGroupPrice{}).
		Distinct("group_name").
		Where("group_name IS NOT NULL AND group_name <> ''").
		Pluck("group_name", &names).Error; err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	now := time.Now().Unix()
	rows := make([]*PricingGroup, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		rows = append(rows, &PricingGroup{
			Name:        n,
			Description: "",
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoNothing: true,
	}).Create(&rows).Error
}

// DeletePricingGroupCascade 删除分组元数据并级联删除所有价格记录
func DeletePricingGroupCascade(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrInvalidPricingGroupName
	}
	tx := DB.Begin()
	if err := tx.Where("group_name = ?", name).Delete(&PricingGroupPrice{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where("name = ?", name).Delete(&PricingGroup{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	return InitPricingGroupPriceCache()
}
