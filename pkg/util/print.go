package util

///黑色 (Black)	  30	40
///红色 (Red)	  31	41
///绿色 (Green)	  32	42
///黄色 (Yellow)  33	43
///蓝色 (Blue)	  34	44
///紫色 (Magenta) 35	45
///青色 (Cyan)	  36	46
///白色 (White)	  37	47

// ANSI color codes
const (
	Reset  = "\033[0m"
	Red    = "\033[1;30;41m"
	Green  = "\033[1;30;42m"
	Yellow = "\033[1;30;43m"
	Blue   = "\033[1;30;44m"
	Purple = "\033[1;30;45m"
	Cyan   = "\033[1;30;46m"
	Gray   = "\033[1;30;47m"
)

// 以下彩色 API 全部接入全局日志系统(util/logger.go):
// 终端保持原有彩色输出, 同时按等级过滤并写入文件(纯文本)。
// 除 LogWithRed 为 Error 级外, 其余均为 Info 级。

func LogWithYellow(tag string, a ...any) { logColored(LevelInfo, Yellow, tag, a...) }
func LogWithCyan(tag string, a ...any)   { logColored(LevelInfo, Cyan, tag, a...) }
func LogWithGray(tag string, a ...any)   { logColored(LevelInfo, Gray, tag, a...) }
func LogWithPurple(tag string, a ...any) { logColored(LevelInfo, Purple, tag, a...) }
func LogWithRed(tag string, a ...any)    { logColored(LevelError, Red, tag, a...) }
func LogWithGreen(tag string, a ...any)  { logColored(LevelInfo, Green, tag, a...) }
func LogWithBlue(tag string, a ...any)   { logColored(LevelInfo, Blue, tag, a...) }
