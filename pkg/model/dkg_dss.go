package model

type PartialSigWrap struct {
	Partial   PriShare
	SessionID []byte
	Signature []byte
}
