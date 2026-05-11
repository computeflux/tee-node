package util

import (
	"encoding/binary"
)

func Int64ToBytes(time int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(time))
	return b
}

func BytesToInt64(b []byte) int64 {
	return int64(binary.BigEndian.Uint64(b))
}

func Uint64ToBytes(time uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, time)
	return b
}

func BytesToUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}
