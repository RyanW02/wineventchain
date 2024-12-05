package retentionpolicy

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/RyanW02/wineventchain/app/internal/utils"
	"github.com/RyanW02/wineventchain/app/pkg/identity"
	"github.com/RyanW02/wineventchain/app/pkg/multiplexer"
	identitytypes "github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	types "github.com/RyanW02/wineventchain/common/pkg/types/retention"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/mitchellh/hashstructure/v2"
	"go.uber.org/zap"
)

type RetentionPolicyApp struct {
	logger     *zap.Logger
	identities identity.Repository
	db         dbm.DB
	policy     *offchain.StoredPolicy
	txState    txState
}

type txState struct {
	policy *offchain.StoredPolicy
}

const dbKey = "policy"

var _ multiplexer.MultiplexedApp = (*RetentionPolicyApp)(nil)

func NewRetentionPolicyApp(logger *zap.Logger, identityRepository identity.Repository, db dbm.DB) (*RetentionPolicyApp, error) {
	app := &RetentionPolicyApp{
		logger:     logger,
		identities: identityRepository,
		db:         db,
		policy:     nil,
		txState:    txState{},
	}

	if err := app.loadPolicy(); err != nil {
		return nil, err
	}

	return app, nil
}

func (app *RetentionPolicyApp) Name() string {
	return types.AppName
}

func (app *RetentionPolicyApp) Info(ctx context.Context, req *abci.RequestInfo) any {
	appHash, err := app.appHash()
	if err != nil {
		app.logger.Warn("Got error getting hash of RetentionPolicyApp", zap.Error(err))
		return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err)
	}

	return map[string]any{
		"app_hash": hex.EncodeToString(appHash),
		"is_set":   app.policy != nil,
	}
}

func (app *RetentionPolicyApp) InitChain(ctx context.Context, req *abci.RequestInitChain) []byte {
	appHash, err := app.appHash()
	if err != nil {
		app.logger.Fatal("Got error getting hash of IdentityApp when running InitChain", zap.Error(err))
	}

	return appHash
}

func (app *RetentionPolicyApp) CheckTx(ctx context.Context, req *abci.RequestCheckTx, data json.RawMessage) (*abci.ResponseCheckTx, error) {
	payload, requester, err := app.decode(data)
	if err != nil {
		return err.IntoCheckTxResponse(), nil
	}

	if requester.Role != identitytypes.RoleAdmin {
		return multiplexer.NewErrorResponse(
			types.CodeUnauthorized,
			types.Codespace,
			errors.New("only principals with the admin role can set the retention policy"),
		).IntoCheckTxResponse(), nil
	}

	if app.policy != nil || app.txState.policy != nil {
		return multiplexer.NewErrorResponse(types.CodePolicyAlreadySet, types.Codespace, errors.New("retention policy already set")).IntoCheckTxResponse(), nil
	}

	switch payload.Type {
	case types.RequestTypeSetPolicy:
		var request types.SetPolicyRequest
		if err := json.Unmarshal(payload.Data, &request); err != nil {
			return multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err).IntoCheckTxResponse(), nil
		}

		if err := request.Policy.Validate(); err != nil {
			return multiplexer.NewErrorResponse(types.CodeInvalidPolicy, types.Codespace, err).IntoCheckTxResponse(), nil
		}
	default:
		return multiplexer.NewErrorResponse(types.CodeUnknownRequestType, types.Codespace, nil).IntoCheckTxResponse(), nil
	}

	return &abci.ResponseCheckTx{
		Code:      types.CodeOk,
		Codespace: types.Codespace,
	}, nil
}

func (app *RetentionPolicyApp) FinalizeBlock(ctx context.Context, req *abci.RequestFinalizeBlock, data json.RawMessage) multiplexer.FinalizeBlockResponse {
	payload, requester, errRes := app.decode(data)
	if errRes != nil {
		return errRes.IntoFinalizeBlockResponse()
	}

	if requester.Role != identitytypes.RoleAdmin {
		return multiplexer.NewErrorResponse(
			types.CodeUnauthorized,
			types.Codespace,
			errors.New("only principals with the admin role can set the retention policy"),
		).IntoFinalizeBlockResponse()
	}

	if app.policy != nil || app.txState.policy != nil {
		return multiplexer.NewErrorResponse(types.CodePolicyAlreadySet, types.Codespace, errors.New("retention policy already set")).IntoFinalizeBlockResponse()
	}

	appHash, err := app.appHash()
	if errRes != nil {
		return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
	}

	switch payload.Type {
	case types.RequestTypeSetPolicy:
		var request types.SetPolicyRequest
		if err := json.Unmarshal(payload.Data, &request); err != nil {
			return multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		if err := request.Policy.Validate(); err != nil {
			return multiplexer.NewErrorResponse(types.CodeInvalidPolicy, types.Codespace, err).IntoFinalizeBlockResponse()
		}

		res, err := json.Marshal(types.SetPolicyResponse{})
		if err != nil {
			return multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		policy := &offchain.StoredPolicy{
			Policy:    request.Policy,
			Author:    payload.Principal,
			AppliedAt: req.Time,
		}
		app.txState.policy = policy

		return multiplexer.FinalizeBlockResponse{
			TxResult: abci.ExecTxResult{
				Code: types.CodeOk,
				Data: res,
				Log:  "policy set",
				Events: []abci.Event{
					utils.Event("policy_set"),
				},
				Codespace: types.Codespace,
			},
			AppHash: appHash,
			CommitFunc: func() error {
				app.policy = policy
				app.txState = txState{}

				marshalled, err := json.Marshal(app.policy)
				if err != nil {
					return err
				}

				return app.db.Set(bz(dbKey), marshalled)
			},
		}
	default:
		return multiplexer.NewErrorResponse(types.CodeUnknownRequestType, types.Codespace, nil).IntoFinalizeBlockResponse()
	}
}

func (app *RetentionPolicyApp) Query(ctx context.Context, req *abci.RequestQuery) (*abci.ResponseQuery, error) {
	if req.Prove {
		return multiplexer.NewErrorResponse(
			types.CodeUnsupportedRequest,
			types.Codespace,
			errors.New("proof operation not valid for RetentionPolicyApp, as there is no merkle tree to provide a proof for"),
		).IntoQueryResponse(), nil
	}

	if app.policy == nil {
		return multiplexer.NewErrorResponse(types.CodePolicyNotSet, types.Codespace, nil).IntoQueryResponse(), nil
	}

	data, err := json.Marshal(app.policy)
	if err != nil {
		return nil, err
	}

	return &abci.ResponseQuery{
		Code:      types.CodeOk,
		Log:       "policy found",
		Value:     data,
		Codespace: types.Codespace,
	}, nil
}

func (app *RetentionPolicyApp) appHash() ([]byte, error) {
	appHash, err := hashstructure.Hash(app.policy, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, err
	}

	appHashBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(appHashBytes, appHash)

	return appHashBytes, nil
}

func (app *RetentionPolicyApp) loadPolicy() error {
	data, err := app.db.Get(bz(dbKey))
	if err != nil {
		return err
	}

	// If data == nil, no policy set
	if data != nil {
		var policy offchain.StoredPolicy
		if err := json.Unmarshal(data, &policy); err != nil {
			return err
		}

		app.policy = &policy
	}

	return nil
}

func bz(s string) []byte {
	return []byte(s)
}

func (app *RetentionPolicyApp) decode(data json.RawMessage) (rpc.SignedPayload, identitytypes.IdentityData, *multiplexer.ErrorResponse) {
	var payload rpc.SignedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		app.logger.Warn("Got error decoding IdentityApp request rpc", zap.Error(err))
		return rpc.SignedPayload{}, identitytypes.IdentityData{}, multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err)
	}

	requester, err := app.identities.Get(payload.Principal)
	if err != nil {
		app.logger.Warn(
			"Got error getting requester identity data",
			zap.Error(err),
			zap.String("requester", payload.Principal.String()),
		)
		return rpc.SignedPayload{}, identitytypes.IdentityData{}, multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err)
	}

	valid, err := payload.ValidateSignature(requester.PublicKey)
	if err != nil {
		app.logger.Warn(
			"Got error validating IdentityApp request signature",
			zap.Error(err),
			zap.String("requester", payload.Principal.String()),
		)
		return rpc.SignedPayload{}, identitytypes.IdentityData{}, multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err)
	}

	if !valid {
		app.logger.Warn(
			"Got invalid IdentityApp request signature",
			zap.String("requester", payload.Principal.String()),
		)
		return rpc.SignedPayload{}, identitytypes.IdentityData{}, multiplexer.NewErrorResponse(rpc.CodeInvalidSignature, rpc.Codespace, nil)
	}

	return payload, requester, nil
}
