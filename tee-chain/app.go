package teechain

import (
	"encoding/binary"
	"errors"

	"github.com/cockroachdb/pebble"
	dbstore "github.com/computeflux/tee-provider/pkg/store"
)

type AppState struct {
	Size           int64
	Height         int64
	RuntimeVersion uint32
}

var stateKey = "appstate"

func (s AppState) Hash() []byte {
	appHash := make([]byte, 8)
	binary.PutVarint(appHash, s.Size)
	return appHash
}

func loadAppState() (AppState, error) {
	state, err := dbstore.GetJson[AppState]("", stateKey)
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return AppState{}, err
	}

	if state == nil {
		state = &AppState{}
	}

	return *state, nil
}

func saveAppState(state *AppState) error {
	return dbstore.SetJson("", stateKey, state)
}
