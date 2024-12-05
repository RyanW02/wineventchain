package multiplexer

import (
	"encoding/json"
	"github.com/RyanW02/wineventchain/app/internal/utils"
	dbm "github.com/cometbft/cometbft-db"
	"sort"
)

type State struct {
	db        dbm.DB
	Size      int64             `json:"size"`
	Height    int64             `json:"height"`
	AppHashes map[string][]byte `json:"app_hash"`
}

func (s State) GenerateAppHash() []byte {
	appNames := utils.Keys(s.AppHashes)
	sort.Strings(appNames)

	var combined []byte
	for _, name := range appNames {
		combined = append(combined, s.AppHashes[name]...)
	}

	return utils.Sha256Sum(combined)
}

func loadState(db dbm.DB) State {
	state := State{
		db: db,
	}

	stateBytes := utils.Must(db.Get(utils.Bytes(stateKey)))
	if len(stateBytes) == 0 {
		state.AppHashes = make(map[string][]byte)
		return state
	}

	if err := json.Unmarshal(stateBytes, &state); err != nil {
		panic(err)
	}

	return state
}

func saveState(state State) {
	stateBytes := utils.Must(json.Marshal(state))

	if err := state.db.Set(utils.Bytes(stateKey), stateBytes); err != nil {
		panic(err)
	}
}
