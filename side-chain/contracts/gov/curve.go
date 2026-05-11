package gov

// Curve 投票曲线，用于计算批准/支持阈值
// 返回值为万分比（10000 = 100%）
//
// 支持三种曲线类型：
//   - LinearDecreasing: 线性递减，从 begin 线性递减到 end
//   - SteppedDecreasing: 阶梯递减，每隔 period 区块下降 step
//   - Reciprocal: 倒数曲线，公式 K/(x/S + x_offset) - y_offset
type Curve struct {
	Type string // 曲线类型：LinearDecreasing, SteppedDecreasing, Reciprocal

	// LinearDecreasing 参数
	// 公式: Y = begin - (begin - end) * x / length
	// 从 (0, begin) 开始，线性递减到 (length, end)，然后保持在 end
	LinearBegin  uint32 // 起始值（万分比）
	LinearEnd    uint32 // 结束值（万分比）
	LinearLength int64  // 线性递减的长度（区块数）

	// SteppedDecreasing 参数
	// 公式: Y = begin - step * floor(x / period)
	// 从 (0, begin) 开始，保持 period 个区块不变，然后下降 step
	StepBegin  uint32 // 起始值（万分比）
	StepEnd    uint32 // 最小值（万分比）
	StepSize   uint32 // 每步下降值（万分比）
	StepPeriod int64  // 每步持续的区块数

	// Reciprocal 参数
	// 公式: Y = K/(x/S + x_offset) - y_offset
	ReciprocalFactor  uint32 // K 常数
	ReciprocalXScale  uint32 // S X轴缩放因子
	ReciprocalXOffset int64  // X轴偏移
	ReciprocalYOffset int64  // Y轴偏移
}

// CurveType 曲线类型常量
const (
	CurveTypeLinearDecreasing  = "LinearDecreasing"  // 线性递减
	CurveTypeSteppedDecreasing = "SteppedDecreasing" // 阶梯递减
	CurveTypeReciprocal        = "Reciprocal"        // 倒数曲线
)

// NewLinearDecreasingCurve 创建线性递减曲线
func NewLinearDecreasingCurve(begin, end uint32, length int64) Curve {
	return Curve{
		Type:         CurveTypeLinearDecreasing,
		LinearBegin:  begin,
		LinearEnd:    end,
		LinearLength: length,
	}
}

// NewSteppedDecreasingCurve 创建阶梯递减曲线
func NewSteppedDecreasingCurve(begin, end, step uint32, period int64) Curve {
	return Curve{
		Type:       CurveTypeSteppedDecreasing,
		StepBegin:  begin,
		StepEnd:    end,
		StepSize:   step,
		StepPeriod: period,
	}
}

// NewReciprocalCurve 创建倒数曲线
func NewReciprocalCurve(factor, xScale uint32, xOffset, yOffset int64) Curve {
	return Curve{
		Type:              CurveTypeReciprocal,
		ReciprocalFactor:  factor,
		ReciprocalXScale:  xScale,
		ReciprocalXOffset: xOffset,
		ReciprocalYOffset: yOffset,
	}
}

// Y 计算曲线在给定 X（区块号）处的 Y 值（阈值，万分比）
// 参数 x: 相对于存款区块的偏移量
// 返回: 阈值百分比（10000 = 100%）
func (c *Curve) Y(x int64) uint32 {
	switch c.Type {
	case CurveTypeLinearDecreasing:
		return c.linearDecreasingY(x)
	case CurveTypeSteppedDecreasing:
		return c.steppedDecreasingY(x)
	case CurveTypeReciprocal:
		return c.reciprocalY(x)
	default:
		return 0
	}
}

func (c *Curve) linearDecreasingY(x int64) uint32 {
	if x >= c.LinearLength {
		return c.LinearEnd
	}
	if c.LinearLength == 0 {
		return c.LinearEnd
	}
	// slope = (begin - end) * x / length
	slope := int64(c.LinearBegin-c.LinearEnd) * x / c.LinearLength
	result := int64(c.LinearBegin) - slope
	if result < int64(c.LinearEnd) {
		return c.LinearEnd
	}
	return uint32(result)
}

func (c *Curve) steppedDecreasingY(x int64) uint32 {
	if c.StepPeriod == 0 || x < c.StepPeriod {
		return c.StepBegin
	}
	numSteps := x / c.StepPeriod
	subValue := numSteps * int64(c.StepSize)

	if subValue >= int64(c.StepBegin) || c.StepBegin-uint32(subValue) <= c.StepEnd {
		return c.StepEnd
	}
	return c.StepBegin - uint32(subValue)
}

func (c *Curve) reciprocalY(x int64) uint32 {
	if c.ReciprocalXScale == 0 {
		return 0
	}
	// K/(x/S + x_offset) - y_offset
	xScale := int64(c.ReciprocalXScale)
	denominator := x/xScale + c.ReciprocalXOffset
	if denominator <= 0 {
		denominator = 1
	}
	result := int64(c.ReciprocalFactor)/denominator - c.ReciprocalYOffset
	if result < 0 {
		return 0
	}
	return uint32(result)
}

// InverseY 反推计算：给定目标阈值 targetY，返回达到该阈值的最小 x（区块偏移量）
// 如果 targetY 大于起始值，返回 0（表示立即满足）
// 如果 targetY 小于结束值，返回 maxOffset（表示永远达不到）
// 注意：需要同时满足 approval 和 support 两个曲线，取较大的 x
func (c *Curve) InverseY(targetY uint32, maxOffset int64) int64 {
	switch c.Type {
	case CurveTypeLinearDecreasing:
		return c.linearDecreasingInverseY(targetY, maxOffset)
	case CurveTypeSteppedDecreasing:
		return c.steppedDecreasingInverseY(targetY, maxOffset)
	case CurveTypeReciprocal:
		return c.reciprocalInverseY(targetY, maxOffset)
	default:
		return maxOffset
	}
}

// linearDecreasingInverseY 线性递减曲线反推
// Y = begin - (begin - end) * x / length
// 反推: x = (begin - Y) * length / (begin - end)
func (c *Curve) linearDecreasingInverseY(targetY uint32, maxOffset int64) int64 {
	// 如果目标值大于等于起始值，立即满足
	if targetY >= c.LinearBegin {
		return 0
	}
	// 如果目标值小于结束值，在曲线范围内永远达不到
	if targetY <= c.LinearEnd {
		return maxOffset
	}
	if c.LinearLength == 0 || c.LinearBegin <= c.LinearEnd {
		return maxOffset
	}

	// x = (begin - targetY) * length / (begin - end)
	numerator := int64(c.LinearBegin-targetY) * c.LinearLength
	denominator := int64(c.LinearBegin - c.LinearEnd)
	x := numerator / denominator

	if x > maxOffset {
		return maxOffset
	}
	return x
}

// steppedDecreasingInverseY 阶梯递减曲线反推
// Y = begin - step * floor(x / period)
// 反推: floor(x / period) = (begin - Y) / step
//
//	x = ceil((begin - Y) / step) * period
func (c *Curve) steppedDecreasingInverseY(targetY uint32, maxOffset int64) int64 {
	// 如果目标值大于等于起始值，立即满足
	if targetY >= c.StepBegin {
		return 0
	}
	// 如果目标值小于等于结束值，在曲线范围内永远达不到
	if targetY <= c.StepEnd {
		return maxOffset
	}
	if c.StepPeriod == 0 || c.StepSize == 0 {
		return maxOffset
	}

	// 需要下降的步数
	diff := c.StepBegin - targetY
	steps := int64(diff) / int64(c.StepSize)
	if int64(diff)%int64(c.StepSize) != 0 {
		steps++ // 向上取整，确保 Y <= targetY
	}

	x := steps * c.StepPeriod
	if x > maxOffset {
		return maxOffset
	}
	return x
}

// reciprocalInverseY 倒数曲线反推
// Y = K/(x/S + x_offset) - y_offset
// 反推: x/S + x_offset = K / (Y + y_offset)
//
//	x = S * (K / (Y + y_offset) - x_offset)
func (c *Curve) reciprocalInverseY(targetY uint32, maxOffset int64) int64 {
	if c.ReciprocalXScale == 0 {
		return maxOffset
	}

	// 如果目标值大于等于 x=0 时的值，立即满足
	yAtZero := c.reciprocalY(0)
	if targetY >= yAtZero {
		return 0
	}

	// 避免除零
	denominator := int64(targetY) + c.ReciprocalYOffset
	if denominator <= 0 {
		return maxOffset
	}

	// x/S + x_offset = K / (Y + y_offset)
	inner := int64(c.ReciprocalFactor) / denominator
	inner -= c.ReciprocalXOffset

	xScale := int64(c.ReciprocalXScale)
	x := inner * xScale

	if x < 0 {
		return 0
	}
	if x > maxOffset {
		return maxOffset
	}
	return x
}
