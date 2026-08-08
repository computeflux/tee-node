package util

// LogError 错误级日志: 终端红色, 同时按全局日志等级过滤并写入文件。
func LogError(tag string, a ...any) {
	logColored(LevelError, Red, tag, a...)
}

// LogOk 信息级日志: 终端绿色, 同时按全局日志等级过滤并写入文件。
func LogOk(tag string, a ...any) {
	logColored(LevelInfo, Green, tag, a...)
}
