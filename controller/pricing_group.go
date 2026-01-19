package controller

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetAllPricingGroups 获取所有定价分组列表
// GET /api/pricing-groups
func GetAllPricingGroups(c *gin.Context) {
	groups, err := model.GetAllPricingGroupNames()
	if err != nil {
		groups = []string{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    groups,
	})
}

// CreatePricingGroup 创建新分组
// POST /api/pricing-groups
func CreatePricingGroup(c *gin.Context) {
	var req struct {
		GroupName   string `json:"group_name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	name := strings.TrimSpace(req.GroupName)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "分组名称不能为空",
		})
		return
	}

	if err := model.CreatePricingGroup(name, req.Description); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "分组创建成功",
	})
}

// DeletePricingGroup 删除分组（级联删除所有价格）
// DELETE /api/pricing-groups/:group
func DeletePricingGroup(c *gin.Context) {
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

	if err := model.DeletePricingGroupCascade(decodedName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "分组删除成功",
	})
}
