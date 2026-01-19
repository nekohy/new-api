package controller

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetAllUserGroups 获取所有用户分组
// 用户分组名称从 user_pricing_access 表中获取 distinct user_group_name
// GET /api/user-groups
func GetAllUserGroups(c *gin.Context) {
	names, err := model.GetAllUserGroupNames()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	data := make([]gin.H, 0, len(names))
	for _, name := range names {
		data = append(data, gin.H{
			"name":        name,
			"description": "",
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// CreateUserGroup 创建用户分组
// POST /api/user-groups
func CreateUserGroup(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "用户分组通过添加模型分组映射自动创建",
	})
}

// UpdateUserGroup 更新用户分组
// PUT /api/user-groups/:group
func UpdateUserGroup(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "用户分组元数据已简化，不再支持描述更新",
	})
}

// DeleteUserGroup 删除用户分组
// DELETE /api/user-groups/:group
func DeleteUserGroup(c *gin.Context) {
	groupName := c.Param("group")
	decoded, err := url.PathUnescape(groupName)
	if err != nil {
		decoded = groupName
	}
	decoded = strings.TrimSpace(decoded)
	if decoded == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "分组名称不能为空"})
		return
	}
	if err := model.ReplaceUserPricingAccessList(decoded, []*model.UserPricingAccess{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "用户分组删除成功"})
}

// GetUserPricingAccessMappings 获取用户分组的模型分组映射
// GET /api/user-groups/:group/model-groups
func GetUserPricingAccessMappings(c *gin.Context) {
	groupName := c.Param("group")
	decoded, err := url.PathUnescape(groupName)
	if err != nil {
		decoded = groupName
	}
	rows, err := model.GetUserPricingAccessList(decoded)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	data := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		if r == nil {
			continue
		}
		data = append(data, gin.H{
			"model_group":     r.ModelGroupName,
			"rate_multiplier": r.RateMultiplier,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// ReplaceUserPricingAccessMappings 替换用户分组的模型分组映射
// PUT /api/user-groups/:group/model-groups
func ReplaceUserPricingAccessMappings(c *gin.Context) {
	groupName := c.Param("group")
	decoded, err := url.PathUnescape(groupName)
	if err != nil {
		decoded = groupName
	}
	var req struct {
		Mappings []struct {
			ModelGroup     string  `json:"model_group"`
			RateMultiplier float64 `json:"rate_multiplier"`
		} `json:"mappings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求参数错误: " + err.Error()})
		return
	}
	mappings := make([]*model.UserPricingAccess, 0, len(req.Mappings))
	for _, m := range req.Mappings {
		mg := strings.TrimSpace(m.ModelGroup)
		if mg == "" {
			continue
		}
		mappings = append(mappings, &model.UserPricingAccess{
			ModelGroupName: mg,
			RateMultiplier: m.RateMultiplier,
		})
	}
	if err := model.ReplaceUserPricingAccessList(decoded, mappings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "count": len(mappings)})
}
