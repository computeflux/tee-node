package util

func LogError(tag string, a ...interface{}) {
	LogWithRed(tag, a...)
}

func LogOk(tag string, a ...interface{}) {
	LogWithGreen(tag, a...)
}
