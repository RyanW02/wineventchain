package rpc

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
)

type Builder struct {
	requestType RequestType
	data        any

	signed    bool
	principal identity.Principal
	key       ed25519.PrivateKey

	app string
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) Data(requestType RequestType, data any) *Builder {
	b.requestType = requestType
	b.data = data
	return b
}

func (b *Builder) App(app string) *Builder {
	b.app = app
	return b
}

func (b *Builder) Signed(principal identity.Principal, key ed25519.PrivateKey) *Builder {
	b.signed = true
	b.principal = principal
	b.key = key
	return b
}

func (b *Builder) Unsigned() *Builder {
	b.signed = false
	return b
}

func (b *Builder) buildSigned() (SignedPayload, error) {
	if b.data == nil || b.requestType == "" {
		return SignedPayload{}, errors.New("Data was not called on builder")
	}

	wrapped, err := wrap(b.requestType, b.data)
	if err != nil {
		return SignedPayload{}, err
	}

	return sign(wrapped, b.principal, b.key)
}

func (b *Builder) buildUnsigned() (UnsignedPayload, error) {
	if b.data == nil || b.requestType == "" {
		return UnsignedPayload{}, errors.New("Data was not called on builder")
	}

	wrapped, err := wrap(b.requestType, b.data)
	if err != nil {
		return UnsignedPayload{}, err
	}

	return unsigned(wrapped), nil
}

func (b *Builder) Build() (MuxedRequest, error) {
	var inner json.RawMessage
	if b.signed {
		payload, err := b.buildSigned()
		if err != nil {
			return MuxedRequest{}, err
		}

		inner, err = json.Marshal(payload)
		if err != nil {
			return MuxedRequest{}, err
		}
	} else {
		payload, err := b.buildUnsigned()
		if err != nil {
			return MuxedRequest{}, err
		}

		inner, err = json.Marshal(payload)
		if err != nil {
			return MuxedRequest{}, err
		}
	}

	return MuxedRequest{
		App:  b.app,
		Data: inner,
	}, nil
}

func (b *Builder) Marshal() ([]byte, error) {
	payload, err := b.Build()
	if err != nil {
		return nil, err
	}

	return json.Marshal(payload)
}
