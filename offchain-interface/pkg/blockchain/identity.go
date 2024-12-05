package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/RyanW02/wineventchain/common/pkg/proof"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	"github.com/pkg/errors"
	"time"
)

var ErrPrincipalNotFound = errors.New("identity not found")

func (c *RoundRobinClient) GetIdentity(principal identity.Principal) (identity.IdentityData, error) {
	conn, err := c.pool.Get()
	if err != nil {
		return identity.IdentityData{}, err
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	// Create data payload to route to the correct sub-app
	data := rpc.MuxedRequest{App: identity.AppName}
	dataMarshalled, err := json.Marshal(data)
	if err != nil {
		return identity.IdentityData{}, err
	}

	res, err := conn.ABCIQuery(ctx, fmt.Sprintf("/%s", principal.String()), dataMarshalled)
	if err != nil {
		return identity.IdentityData{}, err
	}

	if res.Response.Code != identity.CodeOk {
		if res.Response.Codespace == identity.Codespace && res.Response.Code == identity.CodeNotFound {
			return identity.IdentityData{}, ErrPrincipalNotFound
		} else if res.Response.Code != identity.CodeOk {
			return identity.IdentityData{}, fmt.Errorf(
				"unexpected error fetching principal (code %s-%d): %s, %s",
				res.Response.Codespace, res.Response.Code, res.Response.Info, res.Response.Log,
			)
		} else {
			return identity.IdentityData{}, errors.Wrapf(
				ErrABCIQueryFailed,
				"code: %s:%d, log: %s, info: %s",
				res.Response.Codespace, res.Response.Code, res.Response.Log, res.Response.Info,
			)
		}
	}

	var identityData identity.IdentityData
	if err := json.Unmarshal(res.Response.Value, &identityData); err != nil {
		return identity.IdentityData{}, err
	}

	// Validate proof
	if err := proof.ValidateProofOps(res.Response.ProofOps); err != nil {
		return identity.IdentityData{}, err
	}

	return identityData, nil
}
