package identity

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/RyanW02/wineventchain/app/internal/utils"
	"github.com/RyanW02/wineventchain/app/pkg/multiplexer"
	"github.com/RyanW02/wineventchain/common/pkg/proof"
	types "github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/iavl"
	"go.uber.org/zap"
	"strings"
)

type IdentityApp struct {
	Repository
	logger        *zap.Logger
	db            dbm.DB
	versionNumber int64
	txState       txState
}

type txState struct {
	seeded      bool
	registering []types.Principal
}

func defaultTxState() txState {
	return txState{
		seeded:      false,
		registering: make([]types.Principal, 0),
	}
}

var _ multiplexer.MultiplexedApp = (*IdentityApp)(nil)

const treeCacheSize = 1000

func NewIdentityApp(logger *zap.Logger, db dbm.DB) (*IdentityApp, error) {
	tree, err := iavl.NewMutableTree(db, treeCacheSize, false)
	if err != nil {
		return nil, err
	}

	// Load data
	versionNumber, err := tree.Load()
	if err != nil {
		return nil, err
	}

	return &IdentityApp{
		Repository:    NewMerkleRepository(tree),
		logger:        logger,
		db:            db,
		versionNumber: versionNumber,
		txState:       defaultTxState(),
	}, nil
}

func (app *IdentityApp) Name() string {
	return "identity"
}

func (app *IdentityApp) Info(ctx context.Context, req *abci.RequestInfo) any {
	appHash, err := app.Repository.Hash()
	if err != nil {
		app.logger.Warn("Got error getting hash of IdentityApp", zap.Error(err))
		return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err)
	}

	return map[string]any{
		"version":  app.versionNumber,
		"app_hash": hex.EncodeToString(appHash),
	}
}

func (app *IdentityApp) InitChain(ctx context.Context, req *abci.RequestInitChain) []byte {
	appHash, err := app.Repository.Hash()
	if err != nil {
		app.logger.Fatal("Got error getting hash of IdentityApp when running InitChain", zap.Error(err))
	}

	return appHash
}

func (app *IdentityApp) CheckTx(ctx context.Context, req *abci.RequestCheckTx, data json.RawMessage) (*abci.ResponseCheckTx, error) {
	var payload rpc.SignedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		app.logger.Warn("Got error decoding IdentityApp request rpc", zap.Error(err))
		return multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err).IntoCheckTxResponse(), nil
	}

	// The only non-signed rpc is the seed request
	var requester types.IdentityData
	if payload.Type != types.RequestTypeSeed {
		identityData, err := app.extractIdentityData(payload)
		if err != nil {
			return err.IntoCheckTxResponse(), nil
		}

		requester = identityData
	}

	switch payload.Type {
	case types.RequestTypeSeed:
		var seedData types.PayloadSeed
		if err := json.Unmarshal(payload.Data, &seedData); err != nil {
			app.logger.Warn("Got error decoding IdentityApp seed rpc", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err).IntoCheckTxResponse(), nil
		}

		alreadySeeded, err := app.Repository.IsSeeded()
		if err != nil {
			app.logger.Warn("Got error checking if IdentityApp is already seeded", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err).IntoCheckTxResponse(), nil
		}

		if alreadySeeded || app.txState.seeded {
			return multiplexer.NewErrorResponse(types.CodeAlreadySeeded, types.Codespace, errors.New("identity app is already seeded")).IntoCheckTxResponse(), nil
		}

		return &abci.ResponseCheckTx{Code: types.CodeOk, Codespace: types.Codespace}, nil
	case types.RequestTypeRegister:
		if requester.Role != types.RoleAdmin {
			app.logger.Warn(
				"Got non-admin attempting to register",
				zap.String("requester", payload.Principal.String()),
			)
			return multiplexer.NewErrorResponse(types.CodeUnauthorized, types.Codespace, errors.New("only principals with the administrator role can register new principals")).IntoCheckTxResponse(), nil
		}

		var registerData types.PayloadRegister
		if err := json.Unmarshal(payload.Data, &registerData); err != nil {
			app.logger.Warn("Got error decoding request rpc", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err).IntoCheckTxResponse(), nil
		}

		exists, err := app.Repository.Has(registerData.Principal)
		if err != nil {
			app.logger.Warn("Got error checking if principal exists", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err).IntoCheckTxResponse(), nil
		}

		if exists || utils.Contains(app.txState.registering, registerData.Principal) {
			return multiplexer.NewErrorResponse(types.CodePrincipalAlreadyExists, types.Codespace, errors.New("principal already exists")).IntoCheckTxResponse(), nil
		}

		return &abci.ResponseCheckTx{Code: types.CodeOk, Codespace: types.Codespace}, nil
	default:
		app.logger.Warn(
			"Unknown request type",
			zap.String("request_type", string(payload.Type)),
		)
		return multiplexer.NewErrorResponse(types.CodeUnknownRequestType, types.Codespace, nil).IntoCheckTxResponse(), nil
	}
}

func (app *IdentityApp) FinalizeBlock(ctx context.Context, req *abci.RequestFinalizeBlock, data json.RawMessage) multiplexer.FinalizeBlockResponse {
	var payload rpc.SignedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		app.logger.Warn("Got error decoding IdentityApp request rpc", zap.Error(err))
		return multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
	}

	// The only non-signed rpc is the seed request
	var requester types.IdentityData
	if payload.Type != types.RequestTypeSeed {
		identityData, err := app.extractIdentityData(payload)
		if err != nil {
			return err.IntoFinalizeBlockResponse()
		}

		requester = identityData
	}

	switch payload.Type {
	case types.RequestTypeSeed:
		var seedData types.PayloadSeed
		if err := json.Unmarshal(payload.Data, &seedData); err != nil {
			app.logger.Warn("Got error decoding IdentityApp seed rpc", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		alreadySeeded, err := app.Repository.IsSeeded()
		if err != nil {
			app.logger.Warn("Got error checking if IdentityApp is already seeded", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		if alreadySeeded || app.txState.seeded {
			return multiplexer.NewErrorResponse(types.CodeAlreadySeeded, types.Codespace, errors.New("identity app is already seeded")).IntoFinalizeBlockResponse()
		}

		identityData := types.IdentityData{
			PublicKey: seedData.Key,
			Role:      types.RoleAdmin,
		}

		app.txState.seeded = true
		app.txState.registering = append(app.txState.registering, seedData.Principal)

		appHash, err := app.Repository.Hash()
		if err != nil {
			app.logger.Warn("Got error getting app hash", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		return multiplexer.FinalizeBlockResponse{
			TxResult: abci.ExecTxResult{
				Code: types.CodeOk,
			},
			AppHash: appHash,
			CommitFunc: func() error {
				// Reset state to before tx
				app.txState = defaultTxState()

				// Re-apply state
				if err := app.Repository.Store(seedData.Principal, identityData); err != nil {
					return err
				}

				// Re-apply state
				if err := app.Repository.SetSeeded(); err != nil {
					return err
				}

				_, versionNumber, err := app.Repository.Save()
				if err != nil {
					return err
				}

				app.versionNumber = versionNumber
				return err
			},
		}
	case types.RequestTypeRegister:
		if requester.Role != types.RoleAdmin {
			app.logger.Warn(
				"Got non-admin attempting to register",
				zap.String("requester", payload.Principal.String()),
			)
			return multiplexer.NewErrorResponse(types.CodeUnauthorized, types.Codespace, errors.New("only principals with the administrator role can register new principals")).IntoFinalizeBlockResponse()
		}

		var registerData types.PayloadRegister
		if err := json.Unmarshal(payload.Data, &registerData); err != nil {
			app.logger.Warn("Got error decoding request rpc", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		identityData := types.IdentityData{
			PublicKey: registerData.Key,
			Role:      registerData.Role,
		}

		exists, err := app.Repository.Has(registerData.Principal)
		if err != nil {
			app.logger.Warn("Got error checking if principal exists", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		if exists || utils.Contains(app.txState.registering, registerData.Principal) {
			return multiplexer.NewErrorResponse(types.CodePrincipalAlreadyExists, types.Codespace, errors.New("principal already exists")).IntoFinalizeBlockResponse()
		}

		app.txState.registering = append(app.txState.registering, registerData.Principal)

		appHash, err := app.Repository.Hash()
		if err != nil {
			app.logger.Warn("Got error getting app hash", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		return multiplexer.FinalizeBlockResponse{
			TxResult: abci.ExecTxResult{
				Code: types.CodeOk,
			},
			AppHash: appHash,
			CommitFunc: func() error {
				// Reset state to before tx
				app.txState = defaultTxState()

				// Re-apply state
				if err := app.Repository.Store(registerData.Principal, identityData); err != nil {
					return err
				}

				_, versionNumber, err := app.Repository.Save()
				if err != nil {
					return err
				}

				app.versionNumber = versionNumber
				return err
			},
		}
	default:
		app.logger.Warn(
			"Unknown request type",
			zap.String("request_type", string(payload.Type)),
		)
		return multiplexer.NewErrorResponse(types.CodeUnknownRequestType, types.Codespace, nil).IntoFinalizeBlockResponse()
	}
}

func (app *IdentityApp) Query(ctx context.Context, req *abci.RequestQuery) (*abci.ResponseQuery, error) {
	principal := types.Principal(strings.TrimPrefix(req.Path, "/"))

	item, err := app.GetWithProof(principal)
	if err != nil {
		if errors.Is(err, proof.ErrTreeUninitialized) {
			return multiplexer.NewErrorResponse(types.CodeTreeUninitialized, types.Codespace, err).IntoQueryResponse(), nil
		} else {
			app.logger.Error(
				"Got error getting key from IdentityApp",
				zap.Error(err),
				zap.String("principal", principal.String()),
			)
			return multiplexer.NewErrorResponse(types.CodeUnknownError, types.Codespace, err).IntoQueryResponse(), nil
		}
	}

	if item.Item == nil { // Not found
		return &abci.ResponseQuery{
			Code:      types.CodeNotFound,
			Log:       "item not found",
			Index:     item.Index,
			Key:       []byte(principal),
			Value:     nil,
			ProofOps:  item.ProofOps(),
			Height:    item.Height,
			Codespace: types.Codespace,
		}, nil
	} else {
		marshalled, err := json.Marshal(item.Item)
		if err != nil {
			app.logger.Warn("Got error marshalling item", zap.Error(err))
			return nil, err
		}

		return &abci.ResponseQuery{
			Code:      types.CodeOk,
			Index:     item.Index,
			Key:       []byte(principal),
			Value:     marshalled,
			ProofOps:  item.ProofOps(),
			Height:    item.Height,
			Codespace: types.Codespace,
		}, nil
	}
}

func (app *IdentityApp) extractIdentityData(payload rpc.SignedPayload) (types.IdentityData, *multiplexer.ErrorResponse) {
	requester, err := app.Repository.Get(payload.Principal)
	if err != nil {
		app.logger.Warn(
			"Got error getting requester identity data",
			zap.Error(err),
			zap.String("requester", payload.Principal.String()),
		)
		return types.IdentityData{}, multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err)
	}

	valid, err := payload.ValidateSignature(requester.PublicKey)
	if err != nil {
		app.logger.Warn(
			"Got error validating IdentityApp request signature",
			zap.Error(err),
			zap.String("requester", payload.Principal.String()),
		)
		return types.IdentityData{}, multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err)
	}

	if !valid {
		app.logger.Warn(
			"Got invalid IdentityApp request signature",
			zap.String("requester", payload.Principal.String()),
		)
		return types.IdentityData{}, multiplexer.NewErrorResponse(types.CodeInvalidSignature, types.Codespace, nil)
	}

	return requester, nil
}
