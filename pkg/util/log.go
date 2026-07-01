package util

func LogError(tag string, a ...any) {
	LogWithRed(tag, a...)
}

func LogOk(tag string, a ...any) {
	LogWithGreen(tag, a...)
}
