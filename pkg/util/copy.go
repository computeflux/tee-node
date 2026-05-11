package util

import "encoding/json"

func DeepCopy[T any](src T) *T {
	bytes, err := json.Marshal(src)
	if err != nil {
		panic("DeepCopy Marshal" + err.Error())
	}
	dst := new(T)
	err = json.Unmarshal(bytes, dst)
	if err != nil {
		panic("DeepCopy Unmarshal" + err.Error())
	}
	return dst
}
