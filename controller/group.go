package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	// 从新定价系统获取分组列表
	groupNames, err := model.GetAllPricingGroupNames()
	if err != nil {
		groupNames = []string{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userId := c.GetInt("id")
	userGroup, _ := model.GetUserGroup(userId, false)

	// 用户可选的分组：用户分组关联的模型分组列表
	// 下拉框显示的是模型分组，每个模型分组有对应的倍率
	if userGroup != "" {
		// 查询该用户分组关联的模型分组及倍率
		rows, err := model.GetUserPricingAccessList(userGroup)
		if err != nil {
			rows = []*model.UserPricingAccess{}
		}

		// 返回模型分组作为下拉选项，每个模型分组带倍率
		for _, r := range rows {
			if r == nil || r.ModelGroupName == "" {
				continue
			}
			// 获取模型分组的描述
			desc := r.ModelGroupName
			if groupDesc, _ := model.GetPricingGroupDescription(r.ModelGroupName); groupDesc != "" {
				desc = groupDesc
			}
			usableGroups[r.ModelGroupName] = map[string]interface{}{
				"desc":            desc,
				"rate_multiplier": r.RateMultiplier,
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}

// GetUserModelGroupsV2 returns model groups (pricing groups) available to the current user.
//
// This is a new endpoint with clearer semantics than GetUserGroups.
// It returns an array for easier consumption and stable ordering.
//
// Response data:
// [
//
//	{ "name": "groupA", "description": "xxx", "rate_multiplier": 1.0 },
//	...
//
// ]
func GetUserModelGroupsV2(c *gin.Context) {
	type modelGroupItem struct {
		Name           string  `json:"name"`
		Description    string  `json:"description"`
		RateMultiplier float64 `json:"rate_multiplier"`
	}

	userId := c.GetInt("id")
	userGroup, _ := model.GetUserGroup(userId, false)

	items := make([]modelGroupItem, 0)
	if userGroup != "" {
		rows, err := model.GetUserPricingAccessList(userGroup)
		if err != nil {
			rows = []*model.UserPricingAccess{}
		}

		// 收集需要查询的分组名（去重）
		nameSet := make(map[string]bool)
		for _, r := range rows {
			if r != nil && r.ModelGroupName != "" {
				nameSet[r.ModelGroupName] = true
			}
		}
		names := make([]string, 0, len(nameSet))
		for name := range nameSet {
			names = append(names, name)
		}

		// 批量获取描述（消除 N+1 查询）
		descMap, _ := model.GetPricingGroupDescriptions(names)

		// 组装返回数据
		for _, r := range rows {
			if r == nil || r.ModelGroupName == "" {
				continue
			}
			desc := descMap[r.ModelGroupName]
			if desc == "" {
				desc = r.ModelGroupName // 回退到 name
			}
			items = append(items, modelGroupItem{
				Name:           r.ModelGroupName,
				Description:    desc,
				RateMultiplier: r.RateMultiplier,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
	})
}
