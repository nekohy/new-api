package model

import (
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrInvalidRateMultiplier = errors.New("invalid rate multiplier")
var ErrInvalidUserGroupName = errors.New("invalid user group name")

// UserPricingAccess 用户分组 <-> 模型分组 映射 + 倍率
type UserPricingAccess struct {
	ID             uint    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserGroupName  string  `json:"user_group_name" gorm:"type:varchar(64);not null;uniqueIndex:idx_user_model_group"`
	ModelGroupName string  `json:"model_group_name" gorm:"type:varchar(64);not null;uniqueIndex:idx_user_model_group"`
	RateMultiplier float64 `json:"rate_multiplier" gorm:"type:decimal(12,6);not null;default:1.000000"`
	CreatedAt      int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      int64   `json:"updated_at" gorm:"autoUpdateTime"`
}

func (UserPricingAccess) TableName() string {
	return "user_pricing_access"
}

func EnsureUserPricingAccessTable() error {
	// 检查表是否已存在
	if DB.Migrator().HasTable(&UserPricingAccess{}) {
		return nil
	}
	return DB.AutoMigrate(&UserPricingAccess{})
}

type userPricingAccessCache struct {
	mu   sync.RWMutex
	data map[string]map[string]float64 // userGroup -> modelGroup -> multiplier
}

var upaCacheInstance = &userPricingAccessCache{
	data: map[string]map[string]float64{},
}

func InitUserPricingAccessCache() error {
	var rows []*UserPricingAccess
	if err := DB.Find(&rows).Error; err != nil {
		return err
	}
	tmp := make(map[string]map[string]float64)
	for _, r := range rows {
		if r == nil || r.UserGroupName == "" || r.ModelGroupName == "" {
			continue
		}
		if r.RateMultiplier <= 0 {
			continue
		}
		m, ok := tmp[r.UserGroupName]
		if !ok {
			m = make(map[string]float64)
			tmp[r.UserGroupName] = m
		}
		m[r.ModelGroupName] = r.RateMultiplier
	}
	upaCacheInstance.mu.Lock()
	upaCacheInstance.data = tmp
	upaCacheInstance.mu.Unlock()
	return nil
}

func GetUserPricingAccessMultiplier(userGroup, modelGroup string) (float64, bool) {
	upaCacheInstance.mu.RLock()
	defer upaCacheInstance.mu.RUnlock()
	m, ok := upaCacheInstance.data[userGroup]
	if !ok {
		return 1.0, false
	}
	v, ok := m[modelGroup]
	if !ok {
		return 1.0, false
	}
	return v, true
}

func GetUserPricingAccessList(userGroup string) ([]*UserPricingAccess, error) {
	userGroup = strings.TrimSpace(userGroup)
	if userGroup == "" {
		return []*UserPricingAccess{}, nil
	}
	var rows []*UserPricingAccess
	if err := DB.Where("user_group_name = ?", userGroup).
		Order("model_group_name asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ReplaceUserPricingAccessList 全量替换一个用户分组的映射（幂等）
func ReplaceUserPricingAccessList(userGroup string, mappings []*UserPricingAccess) error {
	userGroup = strings.TrimSpace(userGroup)
	if userGroup == "" {
		return ErrInvalidUserGroupName
	}
	now := time.Now().Unix()
	for _, m := range mappings {
		if m == nil {
			continue
		}
		m.UserGroupName = userGroup
		m.ModelGroupName = strings.TrimSpace(m.ModelGroupName)
		if m.ModelGroupName == "" {
			return errors.New("empty model group name")
		}
		if m.RateMultiplier <= 0 {
			return ErrInvalidRateMultiplier
		}
		m.CreatedAt = now
		m.UpdatedAt = now
	}
	tx := DB.Begin()
	if err := tx.Where("user_group_name = ?", userGroup).Delete(&UserPricingAccess{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if len(mappings) > 0 {
		if err := tx.Create(&mappings).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	return InitUserPricingAccessCache()
}

// CheckUserPricingAccess 检查用户分组是否有权访问指定模型
// 返回: (允许访问, 倍率, 错误)
func CheckUserPricingAccess(userGroup, model string) (bool, float64, error) {
	userGroup = strings.TrimSpace(userGroup)
	model = strings.TrimSpace(model)

	if userGroup == "" || model == "" {
		return false, 1.0, errors.New("user group and model cannot be empty")
	}

	// 1. 查询模型所属的模型分组
	modelGroup, ok := GetModelGroupForModel(model)
	if !ok {
		// 模型未分配到任何模型分组，默认拒绝访问
		return false, 1.0, errors.New("model not assigned to any model group")
	}

	// 2. 检查用户分组是否有权访问该模型分组
	multiplier, allowed := GetUserPricingAccessMultiplier(userGroup, modelGroup)
	if !allowed {
		// 用户分组未授权访问该模型分组
		return false, 1.0, nil
	}

	return true, multiplier, nil
}

// GetUserPricingAccessForUserGroups 批量获取多个用户分组的映射
// 返回: userGroup -> []*UserPricingAccess（按 model_group_name 升序）
func GetUserPricingAccessForUserGroups(userGroups []string) (map[string][]*UserPricingAccess, error) {
	if len(userGroups) == 0 {
		return map[string][]*UserPricingAccess{}, nil
	}
	normalized := make([]string, 0, len(userGroups))
	seen := make(map[string]struct{}, len(userGroups))
	for _, g := range userGroups {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if _, ok := seen[g]; ok {
			continue
		}
		seen[g] = struct{}{}
		normalized = append(normalized, g)
	}
	if len(normalized) == 0 {
		return map[string][]*UserPricingAccess{}, nil
	}

	var rows []*UserPricingAccess
	if err := DB.
		Where("user_group_name IN ?", normalized).
		Order("user_group_name asc").
		Order("model_group_name asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[string][]*UserPricingAccess, len(normalized))
	for _, r := range rows {
		if r == nil || r.UserGroupName == "" || r.ModelGroupName == "" {
			continue
		}
		result[r.UserGroupName] = append(result[r.UserGroupName], r)
	}
	return result, nil
}

// GetAllowedModelGroupsForUserGroup 获取用户分组允许访问的所有模型分组
func GetAllowedModelGroupsForUserGroup(userGroup string) ([]string, error) {
	userGroup = strings.TrimSpace(userGroup)
	if userGroup == "" {
		return nil, errors.New("user group cannot be empty")
	}

	upaCacheInstance.mu.RLock()
	defer upaCacheInstance.mu.RUnlock()

	modelGroups, ok := upaCacheInstance.data[userGroup]
	if !ok || len(modelGroups) == 0 {
		return []string{}, nil
	}

	result := make([]string, 0, len(modelGroups))
	for mg := range modelGroups {
		result = append(result, mg)
	}

	return result, nil
}

// GetAllUserGroupNames 获取所有用户分组名称（从 user_pricing_access 表中获取 distinct user_group_name）
func GetAllUserGroupNames() ([]string, error) {
	var names []string
	if err := DB.Model(&UserPricingAccess{}).
		Distinct("user_group_name").
		Order("user_group_name asc").
		Pluck("user_group_name", &names).Error; err != nil {
		return nil, err
	}
	return names, nil
}
