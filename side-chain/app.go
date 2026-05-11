package sidechain

import (
	"encoding/binary"
	"errors"

	"github.com/cockroachdb/pebble"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

type AppState struct {
	Size   int64
	Height int64
}

var stateKey = "appstate"

func (s AppState) Hash() []byte {
	appHash := make([]byte, 8)
	binary.PutVarint(appHash, s.Size)
	return appHash
}

func loadAppState() (AppState, error) {
	state, err := model.GetJson[AppState]("", stateKey)
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return AppState{}, err
	}

	if state == nil {
		state = &AppState{}
	}

	return *state, nil
}

func saveAppState(state *AppState) error {
	return model.SetJson("", stateKey, state)
}
