package multiplexer

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/RyanW02/wineventchain/app/internal/utils"
	common "github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/version"
	"go.uber.org/zap"
	"strconv"
	"strings"
)

const (
	AppVersion      = 1
	stateKey        = "muxer_state"
	validatorPrefix = "val:"
)

var _ types.Application = (*MultiplexedApplication)(nil)

type MultiplexedApplication struct {
	types.BaseApplication

	logger       *zap.Logger
	db           dbm.DB
	apps         map[string]MultiplexedApp
	state        State
	commitFuncs  []func() error
	RetainBlocks int64 // blocks to retain after commit (via ResponseCommit.RetainHeight)

	validators *ValidatorMap
}

func NewApplication(logger *zap.Logger, db dbm.DB, apps ...MultiplexedApp) *MultiplexedApplication {
	state := loadState(db)

	appMap := make(map[string]MultiplexedApp)
	for _, app := range apps {
		appMap[app.Name()] = app
	}

	return &MultiplexedApplication{
		logger:     logger,
		db:         db,
		apps:       appMap,
		state:      state,
		validators: NewValidatorMap(),
	}
}

func (app *MultiplexedApplication) Info(ctx context.Context, req *types.RequestInfo) (*types.ResponseInfo, error) {
	data := make(map[string]any)

	marshalled, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &types.ResponseInfo{
		Data:             string(marshalled),
		Version:          version.ABCIVersion,
		AppVersion:       AppVersion,
		LastBlockHeight:  app.state.Height,
		LastBlockAppHash: app.state.GenerateAppHash(),
	}, nil
}

func (app *MultiplexedApplication) InitChain(ctx context.Context, req *types.RequestInitChain) (*types.ResponseInitChain, error) {
	for _, subApp := range app.apps {
		app.state.AppHashes[subApp.Name()] = subApp.InitChain(ctx, req)
	}

	return &types.ResponseInitChain{
		ConsensusParams: nil,
		Validators:      nil,
		AppHash:         app.state.GenerateAppHash(),
	}, nil
}

func (app *MultiplexedApplication) Query(ctx context.Context, req *types.RequestQuery) (*types.ResponseQuery, error) {
	var decoded common.MuxedRequest
	if err := json.Unmarshal(req.Data, &decoded); err != nil {
		app.logger.Warn("Error decoding CheckTx request", zap.Error(err))
		return NewErrorResponse(CodeEncodingError, Codespace, errors.New("error decoding request")).IntoQueryResponse(), nil
	}

	subApp, ok := app.apps[decoded.App]
	if !ok {
		app.logger.Warn(
			"Got Query request with unknown app name",
			zap.String("supplied_name", decoded.App),
		)
		return NewErrorResponse(CodeUnknownApp, Codespace, errors.New("unknown app name")).IntoQueryResponse(), nil
	}

	return subApp.Query(ctx, req)
}

func (app *MultiplexedApplication) CheckTx(ctx context.Context, req *types.RequestCheckTx) (*types.ResponseCheckTx, error) {
	isValidatorTx := strings.HasPrefix(string(req.Tx), validatorPrefix)
	if isValidatorTx {
		if _, err := parseValidatorTx(string(req.Tx)); err != nil {
			app.logger.Warn("Error parsing validator tx", zap.Error(err))
			return NewErrorResponse(CodeInvalidValidatorTx, Codespace, errors.New("error parsing validator tx")).IntoCheckTxResponse(), nil
		}

		return &types.ResponseCheckTx{Code: CodeOk}, nil
	}

	var decoded common.MuxedRequest
	if err := json.Unmarshal(req.Tx, &decoded); err != nil {
		app.logger.Warn("Error decoding CheckTx request", zap.Error(err))
		return NewErrorResponse(CodeEncodingError, Codespace, errors.New("error decoding request")).IntoCheckTxResponse(), nil
	}

	subApp, ok := app.apps[decoded.App]
	if !ok {
		app.logger.Warn(
			"Got CheckTx request with unknown app name",
			zap.String("supplied_name", decoded.App),
		)
		return NewErrorResponse(CodeUnknownApp, Codespace, errors.New("unknown app name")).IntoCheckTxResponse(), nil
	}

	return subApp.CheckTx(ctx, req, decoded.Data)
}

func (app *MultiplexedApplication) FinalizeBlock(ctx context.Context, req *types.RequestFinalizeBlock) (*types.ResponseFinalizeBlock, error) {
	app.commitFuncs = make([]func() error, 0, len(req.Txs))
	results := make([]*types.ExecTxResult, len(req.Txs))

	// Punish malicious validators
	validatorUpdates := make([]types.ValidatorUpdate, 0, len(req.Misbehavior))
	for _, evidence := range req.Misbehavior {
		app.logger.Warn("Misbehaviour detected", zap.Any("evidence", evidence))

		if evidence.Type == types.MisbehaviorType_DUPLICATE_VOTE {
			address := Address(evidence.Validator.Address)

			validator, ok := app.validators.Get(address)
			if !ok {
				app.logger.Error(
					"Got duplicate vote evidence for unknown validator",
					zap.String("address", address.String()),
				)
				continue
			}

			validatorUpdates = append(validatorUpdates, types.ValidatorUpdate{
				PubKey: validator.PubKey,
				Power:  utils.Max(evidence.Validator.Power-1, 0),
			})
		}
	}

	var events []types.Event
	for i, tx := range req.Txs {
		var decoded common.MuxedRequest
		if err := json.Unmarshal(tx, &decoded); err != nil {
			results[i] = &types.ExecTxResult{Code: CodeEncodingError}
			app.logger.Warn("Error decoding DeliverTx request", zap.Error(err))
			continue
		}

		subApp, ok := app.apps[decoded.App]
		if !ok {
			results[i] = &types.ExecTxResult{Code: CodeUnknownApp}
			app.logger.Warn(
				"Got DeliverTx request with unknown app name",
				zap.String("supplied_name", decoded.App),
			)
			continue
		}

		res := subApp.FinalizeBlock(ctx, req, decoded.Data)
		if res.CommitFunc != nil {
			app.commitFuncs = append(app.commitFuncs, res.CommitFunc)
		}

		events = append(events, res.TxResult.Events...)
		app.state.AppHashes[subApp.Name()] = res.AppHash
		results[i] = &res.TxResult

		app.logger.Info(
			"Ran FinalizeBlock for app",
			zap.String("app", subApp.Name()),
			zap.String("app_hash", hex.EncodeToString(res.AppHash)),
		)
	}

	app.state.Height = req.Height

	res := &types.ResponseFinalizeBlock{
		Events:                nil,
		TxResults:             results,
		ValidatorUpdates:      validatorUpdates,
		ConsensusParamUpdates: nil,
		AppHash:               app.state.GenerateAppHash(),
	}

	app.logger.Info("Finalized block", zap.String("app_hash", hex.EncodeToString(res.AppHash)))

	return res, nil
}

func (app *MultiplexedApplication) Commit(ctx context.Context, commit *types.RequestCommit) (*types.ResponseCommit, error) {
	for _, commitFunc := range app.commitFuncs {
		if err := commitFunc(); err != nil {
			return nil, err
		}
	}

	saveState(app.state)

	resp := &types.ResponseCommit{}
	if app.RetainBlocks > 0 && app.state.Height >= app.RetainBlocks {
		resp.RetainHeight = app.state.Height - app.RetainBlocks + 1
	}

	return resp, nil
}

// Syntax: val:keytype!base64pubkey!power
func parseValidatorTx(tx string) (types.ValidatorUpdate, error) {
	// Trim val: prefix
	tx = strings.TrimPrefix(tx, validatorPrefix)

	parts := strings.Split(tx, "!")
	if len(parts) != 3 {
		return types.ValidatorUpdate{}, fmt.Errorf("invalid validator tx: %s", tx)
	}

	keyType, pubKeyString, powerString := parts[0], parts[1], parts[2]

	var decodedKey []byte
	if _, err := base64.StdEncoding.Decode(decodedKey, []byte(pubKeyString)); err != nil {
		return types.ValidatorUpdate{}, fmt.Errorf("error decoding base64 pubkey: %w", err)
	}

	power, err := strconv.ParseInt(powerString, 10, 64)
	if err != nil {
		return types.ValidatorUpdate{}, fmt.Errorf("error parsing power: %w", err)
	}

	if power < 0 {
		return types.ValidatorUpdate{}, fmt.Errorf("power must be non-negative")
	}

	return types.UpdateValidator(decodedKey, power, keyType), nil
}
