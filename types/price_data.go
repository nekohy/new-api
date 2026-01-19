package types

import "fmt"

// PricingContext 定价上下文
type PricingContext struct {
	UsingGroup string // 使用的分组
}

// PriceSpec 价格规格 ($/1M tokens 或 $/call)
type PriceSpec struct {
	QuotaType         int     // 0=按量计费, 1=按次计费
	InputPrice        float64 // 输入价格 $/1M tokens
	OutputPrice       float64 // 输出价格 $/1M tokens
	CacheReadPrice    float64 // 缓存读取价格 $/1M tokens
	CacheWritePrice   float64 // 缓存写入价格 $/1M tokens
	CacheWrite5mPrice float64 // Claude 5分钟缓存创建价格 $/1M tokens
	CacheWrite1hPrice float64 // Claude 1小时缓存创建价格 $/1M tokens
	ImagePrice        float64 // 图片价格 $/1M tokens
	AudioInputPrice   float64 // 音频输入价格 $/1M tokens
	AudioOutputPrice  float64 // 音频输出价格 $/1M tokens
	FixedPrice        float64 // 固定价格 $/次（按次计费时使用）
}

// CostBreakdown 成本分解（美元）
type CostBreakdown struct {
	TextInputCost    float64 // 文本输入成本
	TextOutputCost   float64 // 文本输出成本
	CacheReadCost    float64 // 缓存读取成本
	CacheWriteCost   float64 // 缓存写入成本
	CacheWrite5mCost float64 // Claude 5分钟缓存创建成本
	CacheWrite1hCost float64 // Claude 1小时缓存创建成本
	AudioInputCost   float64 // 音频输入成本
	AudioOutputCost  float64 // 音频输出成本
	ImageCost        float64 // 图片成本
	FixedCost        float64 // 固定成本
	TotalCost        float64 // 总成本（美元）
	Quota            int     // 转换后的配额
}

// PriceData 定价数据
type PriceData struct {
	FreeModel         bool               // 是否免费模型
	Spec              PriceSpec          // 价格规格
	Cost              CostBreakdown      // 成本分解
	Context           PricingContext     // 定价上下文
	QuotaToPreConsume int                // 预消耗额度
	Multipliers       map[string]float64 // 额外计费因子（如视频秒数、尺寸等）
}

// AddMultiplier 添加计费因子
func (p *PriceData) AddMultiplier(key string, value float64) {
	if p.Multipliers == nil {
		p.Multipliers = make(map[string]float64)
	}
	if value <= 0 {
		return
	}
	p.Multipliers[key] = value
}

// PerCallPriceData 按次计费数据
type PerCallPriceData struct {
	FixedPrice float64        // 固定价格 $/次
	Quota      int            // 配额
	Context    PricingContext // 定价上下文
}

// ToSetting 输出调试信息
func (p *PriceData) ToSetting() string {
	return fmt.Sprintf(
		"Spec[Input: $%.4f/1M, Output: $%.4f/1M, CacheRead: $%.4f/1M, CacheWrite: $%.4f/1M, Fixed: $%.4f/call, QuotaType: %d] "+
			"Cost[Total: $%.6f, Quota: %d] "+
			"Context[Group: %s] PreConsume: %d",
		p.Spec.InputPrice, p.Spec.OutputPrice, p.Spec.CacheReadPrice, p.Spec.CacheWritePrice,
		p.Spec.FixedPrice, p.Spec.QuotaType,
		p.Cost.TotalCost, p.Cost.Quota,
		p.Context.UsingGroup, p.QuotaToPreConsume,
	)
}
