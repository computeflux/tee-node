package dkg

import (
	"github.com/wetee-dao/tee-dsecret/pkg/util"
)

type Logger struct {
	NodeTag string
}

func (l Logger) Info(keyvals ...any) {
	logs := []any{"|| "}
	logs = append(logs, keyvals...)
	util.LogWithGray(l.NodeTag, logs...)
}
func (l Logger) Error(keyvals ...any) {
	logs := []any{"|| "}
	logs = append(logs, keyvals...)
	util.LogWithRed(l.NodeTag, logs...)
}

type NoLogger struct {
}

func (l NoLogger) Info(keyvals ...any) {
}
func (l NoLogger) Error(keyvals ...any) {
}
