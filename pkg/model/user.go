package model

type PublicUser struct {
	// User SS58 address
	Address string `json:"address"`
	// User sign time
	Timestamp int64 `json:"timestamp"`
}
