package util

import (
	"encoding/json"
	"fmt"
)

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

func LogWithYellow(tag string, a ...any) {
	b := make([]any, 0, len(a)+3)

	b = append(b, Yellow+" "+tag)
	b = append(b, Reset)
	b = append(b, a...)

	fmt.Println(b...)
}

func LogWithCyan(tag string, a ...any) {
	b := make([]any, 0, len(a)+3)

	b = append(b, Cyan+" "+tag)
	b = append(b, Reset)
	b = append(b, a...)

	fmt.Println(b...)
}

func LogWithGray(tag string, a ...any) {
	b := make([]any, 0, len(a)+3)

	b = append(b, Gray+" "+tag)
	b = append(b, Reset)
	b = append(b, a...)

	fmt.Println(b...)
}

func LogWithPurple(tag string, a ...any) {
	b := make([]any, 0, len(a)+3)

	b = append(b, Purple+" "+tag)
	b = append(b, Reset)
	b = append(b, a...)

	fmt.Println(b...)
}

func LogWithRed(tag string, a ...any) {
	b := make([]any, 0, len(a)+3)

	b = append(b, Red+" "+tag)
	b = append(b, Reset)
	b = append(b, a...)

	fmt.Println(b...)
}

func LogWithGreen(tag string, a ...any) {
	b := make([]any, 0, len(a)+3)

	b = append(b, Green+" "+tag)
	b = append(b, Reset)
	b = append(b, a...)

	fmt.Println(b...)
}

func LogWithBlue(tag string, a ...any) {
	b := make([]any, 0, len(a)+3)

	b = append(b, Blue+" "+tag)
	b = append(b, Reset)
	b = append(b, a...)

	fmt.Println(b...)
}

func PrintJson(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println("------------------ json --------------------------")
	fmt.Println(string(b))
	fmt.Println("------------------ json --------------------------")
}
