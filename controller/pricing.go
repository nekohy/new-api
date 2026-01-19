package controller

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetPricing(c *gin.Context) {
	pricing := model.GetPricing()
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	// 新定价系统中，分组价格差异通过 group_model_prices 表体现
	// 这里返回所有可用分组，倍率统一为 1.0
	groupRatio := map[string]float64{}
	var group string
	if exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
		}
	}

	usableGroup = service.GetUserUsableGroups(group)
	// 为所有可用分组设置默认倍率 1.0
	for g := range usableGroup {
		groupRatio[g] = 1.0
	}

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        service.GetUserAutoGroup(group),
	})
}
