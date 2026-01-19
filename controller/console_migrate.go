// 用于迁移检测的旧键，该文件下个版本会删除

package controller

import (
	"encoding/json"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// MigrateConsoleSetting 迁移旧的控制台相关配置到 console_setting.*
func MigrateConsoleSetting(c *gin.Context) {
	// 读取全部 option
	opts, err := model.AllOption()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	// 建立 map
	valMap := map[string]string{}
	for _, o := range opts {
		valMap[o.Key] = o.Value
	}

	newOptions := make(map[string]string)
	oldKeys := []string{"ApiInfo", "Announcements", "FAQ", "UptimeKumaUrl", "UptimeKumaSlug"}

	// 1. 处理 APIInfo
	if v := valMap["ApiInfo"]; v != "" {
		var arr []interface{}
		if err := json.Unmarshal([]byte(v), &arr); err == nil {
			if len(arr) > 50 {
				arr = arr[:50]
			}
			bytes, _ := json.Marshal(arr)
			newOptions["console_setting.api_info"] = string(bytes)
		}
	}

	// 2. Announcements 直接搬
	if v := valMap["Announcements"]; v != "" {
		newOptions["console_setting.announcements"] = v
	}

	// 3. FAQ 转换
	if v := valMap["FAQ"]; v != "" {
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(v), &arr); err == nil {
			out := []map[string]interface{}{}
			for _, item := range arr {
				q, _ := item["question"].(string)
				if q == "" {
					q, _ = item["title"].(string)
				}
				a, _ := item["answer"].(string)
				if a == "" {
					a, _ = item["content"].(string)
				}
				if q != "" && a != "" {
					out = append(out, map[string]interface{}{"question": q, "answer": a})
				}
			}
			if len(out) > 50 {
				out = out[:50]
			}
			bytes, _ := json.Marshal(out)
			newOptions["console_setting.faq"] = string(bytes)
		}
	}

	// 4. Uptime Kuma 迁移
	url := valMap["UptimeKumaUrl"]
	slug := valMap["UptimeKumaSlug"]
	if url != "" && slug != "" {
		groups := []map[string]interface{}{
			{
				"id":           1,
				"categoryName": "old",
				"url":          url,
				"slug":         slug,
				"description":  "",
			},
		}
		bytes, _ := json.Marshal(groups)
		newOptions["console_setting.uptime_kuma_groups"] = string(bytes)
	}

	// 使用事务进行原子操作
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		// 写入新配置
		for k, v := range newOptions {
			option := model.Option{Key: k}
			if err := tx.FirstOrCreate(&option, model.Option{Key: k}).Error; err != nil {
				return err
			}
			option.Value = v
			if err := tx.Save(&option).Error; err != nil {
				return err
			}
		}
		// 删除旧键
		return tx.Where("key IN ?", oldKeys).Delete(&model.Option{}).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 重新加载 OptionMap
	model.InitOptionMap()
	common.SysLog("console setting migrated")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "migrated"})
}
