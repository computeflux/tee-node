package scheduler

import "time"

// timeNowUnixNano 是包级时间源的薄封装,方便测试时替换。
var timeNowUnixNano = func() int64 { return time.Now().UnixNano() }
