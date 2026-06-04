package dkgtypes

type PartialSigWrap struct {
	Partial   PriShareWithSuite
	SessionID []byte
	Signature []byte
}
