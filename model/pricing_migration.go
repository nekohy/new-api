package model

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PricingSchemaVersionKey = "PricingSchemaVersion"
	PricingSchemaVersionV2  = 2
)

// GetPricingSchemaVersion 获取当前版本（直接从数据库读取）
func GetPricingSchemaVersion() int {
	var opt Option
	if err := DB.Where("`key` = ?", PricingSchemaVersionKey).First(&opt).Error; err != nil {
		return 1 // 不存在则为 v1
	}
	if opt.Value == "" {
		return 1
	}
	v, _ := strconv.Atoi(opt.Value)
	if v < 1 {
		return 1
	}
	return v
}

func setPricingSchemaVersion(v int) error {
	return UpdateOption(PricingSchemaVersionKey, strconv.Itoa(v))
}

type ModelPair struct {
	Group string
	Model string
}

// normalizeKey 统一字符串清洗逻辑
func normalizeKey(s string) string {
	return strings.TrimSpace(s)
}

// isValidPositiveFloat 统一浮点数验证
func isValidPositiveFloat(v float64) bool {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return false
	}
	return v > 0
}

// loadOptionsByKeys 通用配置批量加载（支持事务）
func loadOptionsByKeys(db *gorm.DB, keys []string) (map[string]string, error) {
	type row struct {
		Key   string
		Value string
	}
	var rows []row
	if err := db.Model(&Option{}).Select("`key`, value").Where("`key` IN ?", keys).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.Value == "" {
			continue
		}
		out[r.Key] = r.Value
	}
	return out, nil
}

func RunPricingMigration() error {
	common.SysLog("===== Pricing Migration Start =====")

	version := GetPricingSchemaVersion()
	common.SysLog(fmt.Sprintf("Current version: %d", version))

	if version < PricingSchemaVersionV2 {
		common.SysLog("Running v1 -> v2 migration...")
		if err := runV1ToV2Migration(); err != nil {
			return fmt.Errorf("v1->v2 migration: %w", err)
		}
		common.SysLog("v1 -> v2 migration done")
	}

	common.SysLog("Running idempotent operations...")
	if err := runIdempotentOps(); err != nil {
		return fmt.Errorf("idempotent ops: %w", err)
	}

	common.SysLog("===== Pricing Migration Complete =====")
	return nil
}

func runV1ToV2Migration() error {
	err := DB.Transaction(func(tx *gorm.DB) error {
		groupRatio, specialGroup, err := loadLegacyConfig(tx)
		if err != nil {
			return err
		}

		userGroups, err := collectUserGroups(tx, groupRatio, specialGroup)
		if err != nil {
			return err
		}
		common.SysLog(fmt.Sprintf("User groups: %v", userGroups))

		rows := buildAccessRows(userGroups, groupRatio, specialGroup)
		common.SysLog(fmt.Sprintf("Built %d access rows", len(rows)))

		if len(rows) > 0 {
			inserted, err := batchInsertAccess(tx, rows)
			if err != nil {
				return err
			}
			common.SysLog(fmt.Sprintf("Inserted %d, skipped %d", inserted, len(rows)-inserted))
		}

		return nil
	})

	if err != nil {
		return err
	}

	// 事务成功后，在事务外更新版本号（避免 SQLite 锁冲突）
	if err := setPricingSchemaVersion(PricingSchemaVersionV2); err != nil {
		return fmt.Errorf("set version v2: %w", err)
	}
	return nil
}

func loadLegacyConfig(db *gorm.DB) (map[string]map[string]float64, map[string]map[string]string, error) {
	groupRatio := make(map[string]map[string]float64)
	specialGroup := make(map[string]map[string]string)

	keys := []string{
		"group_ratio_setting.group_group_ratio",
		"group_ratio_setting.group_special_usable_group",
		"GroupGroupRatio",
		"GroupSpecialUsableGroup",
	}

	kvs, err := loadOptionsByKeys(db, keys)
	if err != nil {
		return nil, nil, err
	}

	if v, ok := kvs["group_ratio_setting.group_group_ratio"]; ok {
		common.SysLog(fmt.Sprintf("  Found key=%s len=%d", "group_ratio_setting.group_group_ratio", len(v)))
		_ = json.Unmarshal([]byte(v), &groupRatio)
	} else if v, ok := kvs["GroupGroupRatio"]; ok {
		common.SysLog(fmt.Sprintf("  Found key=%s len=%d", "GroupGroupRatio", len(v)))
		_ = json.Unmarshal([]byte(v), &groupRatio)
	}
	if v, ok := kvs["group_ratio_setting.group_special_usable_group"]; ok {
		common.SysLog(fmt.Sprintf("  Found key=%s len=%d", "group_ratio_setting.group_special_usable_group", len(v)))
		_ = json.Unmarshal([]byte(v), &specialGroup)
	} else if v, ok := kvs["GroupSpecialUsableGroup"]; ok {
		common.SysLog(fmt.Sprintf("  Found key=%s len=%d", "GroupSpecialUsableGroup", len(v)))
		_ = json.Unmarshal([]byte(v), &specialGroup)
	}

	common.SysLog(fmt.Sprintf("  GroupRatio entries: %d, SpecialGroup entries: %d", len(groupRatio), len(specialGroup)))
	return groupRatio, specialGroup, nil
}

func collectUserGroups(
	db *gorm.DB,
	groupRatio map[string]map[string]float64,
	specialGroup map[string]map[string]string,
) ([]string, error) {
	seen := map[string]bool{"default": true}

	for g := range groupRatio {
		if g = normalizeKey(g); g != "" {
			seen[g] = true
		}
	}
	for g := range specialGroup {
		if g = normalizeKey(g); g != "" {
			seen[g] = true
		}
	}

	// 从 users 表
	var groups []string
	if err := db.Model(&User{}).Distinct("`group`").Where("`group` <> ''").Pluck("`group`", &groups).Error; err != nil {
		return nil, err
	}
	for _, g := range groups {
		if g = normalizeKey(g); g != "" {
			seen[g] = true
		}
	}

	result := make([]string, 0, len(seen))
	for g := range seen {
		result = append(result, g)
	}
	sort.Strings(result)
	return result, nil
}

// buildAccessRows 构建 user_pricing_access 数据
func buildAccessRows(
	userGroups []string,
	groupRatio map[string]map[string]float64,
	specialGroup map[string]map[string]string,
) []*UserPricingAccess {
	// mapping: userGroup -> modelGroup -> multiplier
	mapping := make(map[string]map[string]float64)

	// 初始化：每个用户分组默认可访问 default（倍率 1.0）
	for _, ug := range userGroups {
		mapping[ug] = map[string]float64{"default": 1.0}
	}

	// 处理 specialGroup
	grants := make(map[string]map[string]bool)
	revokes := make(map[string]map[string]bool)

	for ug, ops := range specialGroup {
		ug = normalizeKey(ug)
		if ug == "" {
			continue
		}
		if grants[ug] == nil {
			grants[ug] = make(map[string]bool)
		}
		if revokes[ug] == nil {
			revokes[ug] = make(map[string]bool)
		}

		for key := range ops {
			key = normalizeKey(key)
			var op, mg string
			if strings.HasPrefix(key, "+:") {
				op, mg = "grant", strings.TrimPrefix(key, "+:")
			} else if strings.HasPrefix(key, "-:") {
				op, mg = "revoke", strings.TrimPrefix(key, "-:")
			} else {
				op, mg = "grant", key
			}
			mg = normalizeKey(mg)
			if mg == "" {
				continue
			}
			if op == "grant" {
				grants[ug][mg] = true
			} else {
				revokes[ug][mg] = true
			}
		}
	}

	// 应用 grants
	for ug, mgs := range grants {
		if mapping[ug] == nil {
			mapping[ug] = make(map[string]float64)
		}
		for mg := range mgs {
			if _, ok := mapping[ug][mg]; !ok {
				mapping[ug][mg] = 1.0
			}
		}
	}

	// 应用 revokes
	for ug, mgs := range revokes {
		for mg := range mgs {
			delete(mapping[ug], mg)
		}
	}

	// 应用 groupRatio（仅对已存在的映射）
	for ug, ratios := range groupRatio {
		ug = normalizeKey(ug)
		if ug == "" {
			continue
		}
		for mg, mult := range ratios {
			mg = normalizeKey(mg)
			if mg == "" {
				continue
			}
			if _, ok := mapping[ug][mg]; ok {
				if isValidPositiveFloat(mult) {
					mapping[ug][mg] = mult
				}
			}
		}
	}

	// 转为 slice
	now := time.Now().Unix()
	var result []*UserPricingAccess
	for ug, mgs := range mapping {
		for mg, mult := range mgs {
			result = append(result, &UserPricingAccess{
				UserGroupName:  ug,
				ModelGroupName: mg,
				RateMultiplier: mult,
				CreatedAt:      now,
				UpdatedAt:      now,
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].UserGroupName == result[j].UserGroupName {
			return result[i].ModelGroupName < result[j].ModelGroupName
		}
		return result[i].UserGroupName < result[j].UserGroupName
	})

	return result
}

// batchInsertAccess 批量插入（ON CONFLICT DO NOTHING）
func batchInsertAccess(db *gorm.DB, rows []*UserPricingAccess) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_group_name"}, {Name: "model_group_name"}},
		DoNothing: true,
	}).CreateInBatches(&rows, 100)

	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

func runIdempotentOps() error {
	if err := ensureDefaultGroup(DB); err != nil {
		return err
	}

	// 从 abilities 回填 pricing_group_prices
	if err := backfillPrices(DB); err != nil {
		common.SysError("backfillPrices: " + err.Error())
		// 不阻止启动
	}

	return nil
}

func ensureDefaultGroup(db *gorm.DB) error {
	// pricing_groups: default
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoNothing: true,
	}).Create(&PricingGroup{
		Name:        "default",
		Description: "默认模型分组",
	}).Error; err != nil {
		return err
	}

	// user_pricing_access: default -> default (1.0)
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_group_name"}, {Name: "model_group_name"}},
		DoNothing: true,
	}).Create(&UserPricingAccess{
		UserGroupName:  "default",
		ModelGroupName: "default",
		RateMultiplier: 1.0,
	}).Error; err != nil {
		return err
	}

	return nil
}

func backfillPrices(db *gorm.DB) error {
	// 从旧倍率配置计算价格
	legacy, err := loadLegacyPricing(db)
	if err != nil {
		return err
	}
	if legacy == nil {
		return nil
	}

	// 获取 abilities 中的 group+model 对
	var pairs []ModelPair
	if err := db.Model(&Ability{}).
		Select("DISTINCT `group`, model").
		Where("`group` <> '' AND model <> ''").
		Scan(&pairs).Error; err != nil {
		return err
	}

	if len(pairs) == 0 {
		return nil
	}

	const basePricePer1M = 2.0
	now := common.GetTimestamp()

	// 使用批量 Upsert 替代 N+1 查询
	return backfillPricesBatchUpsert(db, legacy, pairs, basePricePer1M, now)
}

type legacyPricing struct {
	ModelRatio      map[string]float64
	CompletionRatio map[string]float64
	CacheRatio      map[string]float64
	ModelPrice      map[string]float64
	GroupRatio      map[string]float64
}

func loadLegacyPricing(db *gorm.DB) (*legacyPricing, error) {
	lp := &legacyPricing{
		ModelRatio:      make(map[string]float64),
		CompletionRatio: make(map[string]float64),
		CacheRatio:      make(map[string]float64),
		ModelPrice:      make(map[string]float64),
		GroupRatio:      make(map[string]float64),
	}

	keys := []string{"ModelRatio", "CompletionRatio", "CacheRatio", "ModelPrice", "GroupRatio"}
	kvs, err := loadOptionsByKeys(db, keys)
	if err != nil {
		return nil, err
	}
	if v, ok := kvs["ModelRatio"]; ok {
		_ = json.Unmarshal([]byte(v), &lp.ModelRatio)
	}
	if v, ok := kvs["CompletionRatio"]; ok {
		_ = json.Unmarshal([]byte(v), &lp.CompletionRatio)
	}
	if v, ok := kvs["CacheRatio"]; ok {
		_ = json.Unmarshal([]byte(v), &lp.CacheRatio)
	}
	if v, ok := kvs["ModelPrice"]; ok {
		_ = json.Unmarshal([]byte(v), &lp.ModelPrice)
	}
	if v, ok := kvs["GroupRatio"]; ok {
		_ = json.Unmarshal([]byte(v), &lp.GroupRatio)
	}
	return lp, nil
}

func computePrice(lp *legacyPricing, group, model string, basePricePer1M float64, now int64) *PricingGroupPrice {
	if lp == nil || group == "" || model == "" {
		return nil
	}

	modelRatio := 1.0
	if v, ok := lp.ModelRatio[model]; ok {
		modelRatio = v
	}
	completionRatio := 1.0
	if v, ok := lp.CompletionRatio[model]; ok {
		completionRatio = v
	}
	cacheRatio := 0.1
	if v, ok := lp.CacheRatio[model]; ok {
		cacheRatio = v
	}
	groupRatio := 1.0
	if v, ok := lp.GroupRatio[group]; ok {
		groupRatio = v
	}

	inputPrice := modelRatio * basePricePer1M * groupRatio
	outputPrice := inputPrice * completionRatio
	cachePrice := inputPrice * cacheRatio

	if math.IsNaN(inputPrice) || math.IsInf(inputPrice, 0) || inputPrice < 0 {
		return nil
	}

	var fixedPrice *float64
	var quotaType *int
	if v, ok := lp.ModelPrice[model]; ok {
		fp := v * groupRatio
		if !math.IsNaN(fp) && !math.IsInf(fp, 0) && fp >= 0 {
			fixedPrice = &fp
			qt := 1
			quotaType = &qt
		}
	}

	return &PricingGroupPrice{
		GroupName:   group,
		Model:       model,
		InputPrice:  &inputPrice,
		OutputPrice: &outputPrice,
		CachePrice:  &cachePrice,
		FixedPrice:  fixedPrice,
		QuotaType:   quotaType,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func upsertPriceCoalesce(row *PricingGroupPrice) {
	var existing PricingGroupPrice
	err := DB.Where("group_name = ? AND model = ?", row.GroupName, row.Model).First(&existing).Error
	if err != nil {
		// 不存在，创建
		DB.Create(row)
		return
	}

	// 存在，只更新 nil 字段（COALESCE 逻辑）
	updates := map[string]interface{}{"updated_at": row.UpdatedAt}
	if existing.InputPrice == nil && row.InputPrice != nil {
		updates["input_price"] = row.InputPrice
	}
	if existing.OutputPrice == nil && row.OutputPrice != nil {
		updates["output_price"] = row.OutputPrice
	}
	if existing.CachePrice == nil && row.CachePrice != nil {
		updates["cache_price"] = row.CachePrice
	}
	if existing.FixedPrice == nil && row.FixedPrice != nil {
		updates["fixed_price"] = row.FixedPrice
	}
	if existing.QuotaType == nil && row.QuotaType != nil {
		updates["quota_type"] = row.QuotaType
	}

	if len(updates) > 1 {
		DB.Model(&PricingGroupPrice{}).
			Where("group_name = ? AND model = ?", row.GroupName, row.Model).
			Updates(updates)
	}
}

// backfillPricesBatchUpsert 批量 Upsert 优化（避免 N+1 查询）
// 支持多数据库方言：PostgreSQL, MySQL, SQLite
func backfillPricesBatchUpsert(db *gorm.DB, legacy *legacyPricing, pairs []ModelPair, basePricePer1M float64, now int64) error {
	// 1) 计算候选 rows（内存）
	rows := make([]*PricingGroupPrice, 0, len(pairs))
	type key struct {
		Group string
		Model string
	}
	keys := make([]key, 0, len(pairs))
	for _, p := range pairs {
		row := computePrice(legacy, p.Group, p.Model, basePricePer1M, now)
		if row == nil {
			continue
		}
		rows = append(rows, row)
		keys = append(keys, key{Group: row.GroupName, Model: row.Model})
	}
	if len(rows) == 0 {
		return nil
	}

	// 2) 批量查询 existing（避免 N+1）
	var existing []PricingGroupPrice
	// 使用 IN 查询批量获取已存在的记录
	conditions := make([]string, len(keys))
	args := make([]interface{}, 0, len(keys)*2)
	for i, k := range keys {
		conditions[i] = "(group_name = ? AND model = ?)"
		args = append(args, k.Group, k.Model)
	}
	query := strings.Join(conditions, " OR ")
	if err := db.Model(&PricingGroupPrice{}).
		Select("group_name, model, input_price, output_price, cache_price, fixed_price, quota_type").
		Where(query, args...).
		Find(&existing).Error; err != nil {
		return err
	}

	existingMap := make(map[key]PricingGroupPrice, len(existing))
	for _, e := range existing {
		existingMap[key{Group: e.GroupName, Model: e.Model}] = e
	}

	// 3) 构建需要写入的 rows（保持"只填 nil"语义）
	toInsert := make([]*PricingGroupPrice, 0)
	toUpdate := make([]struct {
		row     *PricingGroupPrice
		updates map[string]interface{}
	}, 0)

	for _, r := range rows {
		k := key{Group: r.GroupName, Model: r.Model}
		e, ok := existingMap[k]
		if !ok {
			// 不存在，准备插入
			toInsert = append(toInsert, r)
			continue
		}

		// 存在，检查哪些字段需要更新（只填 nil）
		updates := map[string]interface{}{"updated_at": r.UpdatedAt}
		changed := false
		if e.InputPrice == nil && r.InputPrice != nil {
			updates["input_price"] = r.InputPrice
			changed = true
		}
		if e.OutputPrice == nil && r.OutputPrice != nil {
			updates["output_price"] = r.OutputPrice
			changed = true
		}
		if e.CachePrice == nil && r.CachePrice != nil {
			updates["cache_price"] = r.CachePrice
			changed = true
		}
		if e.FixedPrice == nil && r.FixedPrice != nil {
			updates["fixed_price"] = r.FixedPrice
			changed = true
		}
		if e.QuotaType == nil && r.QuotaType != nil {
			updates["quota_type"] = r.QuotaType
			changed = true
		}

		if changed {
			toUpdate = append(toUpdate, struct {
				row     *PricingGroupPrice
				updates map[string]interface{}
			}{row: r, updates: updates})
		}
	}

	// 4) 批量插入新记录
	if len(toInsert) > 0 {
		if err := db.CreateInBatches(&toInsert, 200).Error; err != nil {
			return err
		}
		common.SysLog(fmt.Sprintf("  Inserted %d new prices", len(toInsert)))
	}

	// 5) 批量更新已存在记录（比原来的 N+1 查询效率高）
	if len(toUpdate) > 0 {
		for _, item := range toUpdate {
			if err := db.Model(&PricingGroupPrice{}).
				Where("group_name = ? AND model = ?", item.row.GroupName, item.row.Model).
				Updates(item.updates).Error; err != nil {
				return err
			}
		}
		common.SysLog(fmt.Sprintf("  Updated %d existing prices", len(toUpdate)))
	}

	return nil
}

func initCaches() error {
	if err := InitPricingGroupPriceCache(); err != nil {
		return fmt.Errorf("pricing group price cache: %w", err)
	}
	if err := InitUserPricingAccessCache(); err != nil {
		return fmt.Errorf("user pricing access cache: %w", err)
	}
	if err := InitPricingGroupModelCache(); err != nil {
		return fmt.Errorf("pricing group model cache: %w", err)
	}
	return nil
}
