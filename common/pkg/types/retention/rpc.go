package retention

import (
	"github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/google/uuid"
)

const (
	AppName = "retention_policy"

	// RequestTypeSetPolicy is used to set the chain's retention policy
	RequestTypeSetPolicy = "set_policy"
)

type SetPolicyRequest struct {
	Policy offchain.RetentionPolicy `json:"policy"`
	Nonce  uuid.UUID                `json:"nonce"`
}

type SetPolicyResponse struct{}
