package model

type NewEpochMsg struct {
	Time int64
	// MapAccountPartialSig []byte
	PartialSig []byte
}
