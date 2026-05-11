package util

func LogError(tag string, a ...interface{}) {
	LogWithRed(tag, a...)
}

func LogOk(tag string, a ...interface{}) {
	LogWithGreen(tag, a...)
}

func LogSendmsg(tag string, a ...interface{}) {
	LogWithCyan(tag, a...)
}

func LogRevmsg(tag string, a ...interface{}) {
	LogWithPurple(tag, a...)
}
