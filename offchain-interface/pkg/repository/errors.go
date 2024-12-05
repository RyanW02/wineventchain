package repository

import "github.com/pkg/errors"

var (
	ErrEventAlreadyStored = errors.New("event already stored")
	ErrInvalidFilter      = errors.New("invalid filter")
)
