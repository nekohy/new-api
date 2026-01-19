package service

import (
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	// 新定价系统：返回用户分组可以访问的模型分组列表
	result := make(map[string]string)

	if userGroup != "" {
		// 从 user_group_model_groups 表获取该用户分组可以访问的模型分组
		allowedModelGroups, err := model.GetAllowedModelGroupsForUserGroup(userGroup)
		if err != nil {
			common.SysError("GetUserUsableGroups: failed to get allowed model groups: " + err.Error())
		} else {
			// 获取每个模型分组的描述
			for _, mg := range allowedModelGroups {
				desc, _ := model.GetPricingGroupDescription(mg)
				if desc == "" {
					desc = mg
				}
				result[mg] = desc
			}
		}
	}

	// 添加内置的 auto 分组（如果配置中存在）
	builtinGroups := setting.GetUserUsableGroupsCopy()
	if desc, ok := builtinGroups["auto"]; ok {
		result["auto"] = desc
	}

	return result
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
// 返回用户可用分组列表，按各分组的平均价格从低到高排序
// 这样在 auto 模式下会优先使用价格最低的分组
func GetUserAutoGroup(userGroup string) []string {
	groups := GetUserUsableGroups(userGroup)
	autoGroups := make([]string, 0)

	// 收集用户可用的分组（排除 auto 本身）
	for _, group := range setting.GetAutoGroups() {
		if group == "auto" {
			continue
		}
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}

	// 如果只有一个或没有分组，无需排序
	if len(autoGroups) <= 1 {
		return autoGroups
	}

	// 按分组平均价格升序排序
	// 价格计算：取分组内所有模型的 InputPrice 平均值作为排序依据
	sort.Slice(autoGroups, func(i, j int) bool {
		priceI := getGroupAveragePrice(autoGroups[i])
		priceJ := getGroupAveragePrice(autoGroups[j])
		return priceI < priceJ
	})

	return autoGroups
}

// getGroupAveragePrice 计算分组的平均输入价格
// 用于 auto 分组的价格排序，价格越低排序越靠前
func getGroupAveragePrice(groupName string) float64 {
	prices, err := model.GetGroupAllPrices(groupName)
	if err != nil || len(prices) == 0 {
		// 无价格配置时返回最大值，排到最后
		return 1e18
	}

	var total float64
	var count int
	for _, p := range prices {
		if p != nil && p.InputPrice != nil {
			total += *p.InputPrice
			count++
		}
	}

	if count == 0 {
		return 1e18
	}
	return total / float64(count)
}
