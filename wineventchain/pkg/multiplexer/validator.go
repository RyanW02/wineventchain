package multiplexer

import (
	"github.com/cometbft/cometbft/libs/sync"
	"github.com/cometbft/cometbft/proto/tendermint/crypto"
)

type ValidatorMap struct {
	mu    sync.RWMutex
	inner map[Address]Validator
}

type Validator struct {
	Address Address
	PubKey  crypto.PublicKey
}

type Address string

func (a Address) String() string {
	return string(a)
}

func NewValidatorMap() *ValidatorMap {
	return &ValidatorMap{
		mu:    sync.RWMutex{},
		inner: make(map[Address]Validator),
	}
}

func (v *ValidatorMap) Add(val Validator) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.inner[val.Address] = val
}

func (v *ValidatorMap) Get(address Address) (Validator, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	val, ok := v.inner[address]
	return val, ok
}
