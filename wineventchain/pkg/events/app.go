package events

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/RyanW02/wineventchain/app/internal/utils"
	"github.com/RyanW02/wineventchain/app/pkg/identity"
	"github.com/RyanW02/wineventchain/app/pkg/multiplexer"
	"github.com/RyanW02/wineventchain/common/pkg/proof"
	types "github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/iavl"
	"go.uber.org/zap"
	"regexp"
)

type EventsApp struct {
	Repository    Repository
	logger        *zap.Logger
	identities    identity.Repository
	versionNumber int64
	txState       txState
}

type txState struct {
	creating []string
}

func defaultTxState() txState {
	return txState{
		creating: make([]string, 0),
	}
}

var _ multiplexer.MultiplexedApp = (*EventsApp)(nil)

const (
	treeCacheSize = 1000
)

func NewEventsApp(logger *zap.Logger, db dbm.DB, identityRepository identity.Repository) (*EventsApp, error) {
	tree, err := iavl.NewMutableTree(db, treeCacheSize, false)
	if err != nil {
		return nil, err
	}

	// Load data
	versionNumber, err := tree.Load()
	if err != nil {
		return nil, err
	}

	return &EventsApp{
		Repository:    NewMerkleRepository(tree),
		logger:        logger,
		identities:    identityRepository,
		versionNumber: versionNumber,
		txState:       defaultTxState(),
	}, nil
}

func (app *EventsApp) Name() string {
	return types.AppName
}

func (app *EventsApp) Info(ctx context.Context, req *abci.RequestInfo) any {
	appHash, err := app.Repository.Hash()
	if err != nil {
		app.logger.Warn("Got error getting hash of EventsApp", zap.Error(err))
		return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err)
	}

	return map[string]any{
		"version":  app.versionNumber,
		"app_hash": hex.EncodeToString(appHash),
	}
}

func (app *EventsApp) InitChain(ctx context.Context, req *abci.RequestInitChain) []byte {
	appHash, err := app.Repository.Hash()
	if err != nil {
		app.logger.Fatal("Got error getting hash of EventsApp when running InitChain", zap.Error(err))
	}

	return appHash
}

func (app *EventsApp) CheckTx(ctx context.Context, req *abci.RequestCheckTx, data json.RawMessage) (*abci.ResponseCheckTx, error) {
	// Checks the signature on the request, and ensure that the principal making the request exists
	decoded, err := app.decode(data)
	if err != nil {
		return err.IntoCheckTxResponse(), nil
	}

	switch decoded.Type {
	case types.RequestTypeCreate:
		var payload types.CreateRequest
		if err := json.Unmarshal(decoded.Data, &payload); err != nil {
			app.logger.Warn("Got error decoding EventsApp create payload", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err).IntoCheckTxResponse(), nil
		}
	default:
		return multiplexer.NewErrorResponse(
			rpc.CodeUnknownRequestType,
			rpc.Codespace,
			fmt.Errorf("unknown request type: %s", decoded.Type),
		).IntoCheckTxResponse(), nil
	}

	return &abci.ResponseCheckTx{
		Code:      multiplexer.CodeOk,
		Codespace: multiplexer.Codespace,
	}, nil
}

func (app *EventsApp) FinalizeBlock(ctx context.Context, req *abci.RequestFinalizeBlock, data json.RawMessage) multiplexer.FinalizeBlockResponse {
	decoded, err := app.decode(data)
	if err != nil {
		return err.IntoFinalizeBlockResponse()
	}
	if err := app.Repository.LoadVersion(app.versionNumber); err != nil {
		app.logger.Warn("Got error loading EventsApp version", zap.Error(err))
		return multiplexer.NewErrorResponse(rpc.CodeUnknownRequestType, rpc.Codespace, err).IntoFinalizeBlockResponse()
	}

	switch decoded.Type {
	case types.RequestTypeCreate:
		var payload types.CreateRequest
		if err := json.Unmarshal(decoded.Data, &payload); err != nil {
			app.logger.Warn("Got error decoding EventsApp create payload", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		eventId, err := types.NewEventHash(uint64(req.Height), decoded.Principal, payload.Event.Event)
		if err != nil {
			app.logger.Error("Failed to generate EventId hash", zap.Error(err), zap.Int64("height", req.Height))
			return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		// De-dup
		if utils.Contains(app.txState.creating, eventId.String()) {
			app.logger.Warn("Duplicate event creation request", zap.Stringer("event_id", eventId))
			return multiplexer.NewErrorResponse(types.CodeUnknownError, types.Codespace, nil).IntoFinalizeBlockResponse()
		}

		app.txState.creating = append(app.txState.creating, eventId.String())

		metadata := types.Metadata{
			EventId:      eventId,
			ReceivedTime: req.Time, // Deterministic
			Principal:    decoded.Principal,
		}

		appHash, err := app.Repository.Hash()
		if err != nil {
			app.logger.Warn("Got error getting app hash", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		app.logger.Info(
			"Generated pre-commit AppHash for event create",
			zap.String("app_hash", hex.EncodeToString(appHash)),
			zap.Stringer("event_id", eventId),
		)

		responseMarshalled, err := json.Marshal(types.CreateResponse{Metadata: metadata})
		if err != nil {
			app.logger.Warn("Got error marshalling create response", zap.Error(err))
			return multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err).IntoFinalizeBlockResponse()
		}

		return multiplexer.FinalizeBlockResponse{
			TxResult: abci.ExecTxResult{
				Code: types.CodeOk,
				Data: responseMarshalled,
				Log:  "event stored",
				Events: []abci.Event{utils.Event(types.EventCreate,
					abci.EventAttribute{
						Key:   types.AttributeType,
						Value: types.AttributeValueCreate,
						Index: true,
					},
					abci.EventAttribute{
						Key:   types.AttributeEventId,
						Value: eventId.String(),
						Index: true,
					},
					abci.EventAttribute{
						Key:   types.AttributePrincipal,
						Value: decoded.Principal.String(),
						Index: true,
					},
				)},
				Codespace: types.Codespace,
			},
			AppHash: appHash,
			CommitFunc: func() error {
				app.logger.Info("Committing event", zap.Stringer("event_id", eventId))

				// Reset state to before tx
				app.txState = defaultTxState()

				// Apply state changes
				if err := app.Repository.Store(types.EventWithMetadata{
					ScrubbedEvent: payload.Event,
					Metadata:      metadata,
				}); err != nil {
					return err
				}

				newHash, versionNumber, err := app.Repository.Save()
				if err != nil {
					return err
				}

				app.logger.Info(
					"Committed event successfully",
					zap.Stringer("event_id", eventId),
					zap.String("app_hash", hex.EncodeToString(newHash)),
					zap.Int("version_number", int(versionNumber)),
				)

				app.versionNumber = versionNumber
				return nil
			},
		}
	default:
		return multiplexer.NewErrorResponse(
			rpc.CodeUnknownRequestType,
			rpc.Codespace,
			fmt.Errorf("unknown request type: %s", decoded.Type),
		).IntoFinalizeBlockResponse()
	}
}

// /event-by-id/{event_id} where event_id is a hex encoded sha256 hash
var pathRegex = regexp.MustCompile(`^/event-by-id/([a-f0-9]{64})$`)

func (app *EventsApp) Query(ctx context.Context, req *abci.RequestQuery) (*abci.ResponseQuery, error) {
	if req.Path == "/count" {
		count, err := app.Repository.EventCount()
		if err != nil {
			app.logger.Error("Got error getting event count", zap.Error(err))
			return multiplexer.NewErrorResponse(types.CodeUnknownError, types.Codespace, err).IntoQueryResponse(), nil
		}

		marshalled, err := json.Marshal(types.EventCountResponse{Count: count})
		if err != nil {
			app.logger.Error("Got error marshalling event count", zap.Error(err))
			return multiplexer.NewErrorResponse(types.CodeUnknownError, types.Codespace, err).IntoQueryResponse(), nil
		}

		return &abci.ResponseQuery{
			Code:      types.CodeOk,
			Log:       "event count",
			Height:    req.Height,
			Value:     marshalled,
			Codespace: types.Codespace,
		}, nil
	}

	match := pathRegex.FindStringSubmatch(req.Path)
	if len(match) != 2 {
		return multiplexer.NewErrorResponse(types.CodeInvalidQueryPath, types.Codespace, nil).IntoQueryResponse(), nil
	}

	eventIdRaw, err := hex.DecodeString(match[1])
	if err != nil { // *Should* be infallible
		return multiplexer.NewErrorResponse(types.CodeInvalidQueryPath, types.Codespace, nil).IntoQueryResponse(), nil
	}

	eventId := types.EventHash(eventIdRaw)

	ev, err := app.Repository.GetWithProof(eventId)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return multiplexer.NewErrorResponse(types.CodeEventNotFound, types.Codespace, err).IntoQueryResponse(), nil
		} else if errors.Is(err, proof.ErrTreeUninitialized) {
			return multiplexer.NewErrorResponse(types.CodeTreeUninitialized, types.Codespace, err).IntoQueryResponse(), nil
		} else {
			app.logger.Error("Got error getting event with proof", zap.Error(err), zap.Stringer("event_id", eventId))
			return multiplexer.NewErrorResponse(types.CodeUnknownError, types.Codespace, err).IntoQueryResponse(), nil
		}
	}

	height := req.Height
	if height == 0 {
		height = ev.Height
	}

	if ev.Item == nil {
		return &abci.ResponseQuery{
			Code:      types.CodeEventNotFound,
			Log:       "Event not found",
			Index:     ev.Index,
			Key:       eventId,
			Value:     nil,
			ProofOps:  ev.ProofOps(),
			Height:    height,
			Codespace: types.Codespace,
		}, nil
	}

	marshalled, err := json.Marshal(ev.Item)
	if err != nil {
		app.logger.Error("Got error marshalling event item", zap.Error(err), zap.Stringer("event_id", eventId))
		return multiplexer.NewErrorResponse(types.CodeUnknownError, types.Codespace, err).IntoQueryResponse(), nil
	}

	return &abci.ResponseQuery{
		Code:      types.CodeOk,
		Log:       "Event found",
		Index:     ev.Index,
		Key:       eventId,
		Value:     marshalled,
		ProofOps:  ev.ProofOps(),
		Height:    height,
		Codespace: types.Codespace,
	}, nil
}

func (app *EventsApp) decode(data json.RawMessage) (rpc.SignedPayload, *multiplexer.ErrorResponse) {
	var payload rpc.SignedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		app.logger.Warn("Got error decoding EventsApp request rpc", zap.Error(err))
		return rpc.SignedPayload{}, multiplexer.NewErrorResponse(multiplexer.CodeEncodingError, multiplexer.Codespace, err)
	}

	requester, err := app.identities.Get(payload.Principal)
	if err != nil {
		app.logger.Warn(
			"Got error getting requester identity data",
			zap.Error(err),
			zap.String("requester", payload.Principal.String()),
		)
		return rpc.SignedPayload{}, multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err)
	}

	valid, err := payload.ValidateSignature(requester.PublicKey)
	if err != nil {
		app.logger.Warn(
			"Got error validating EventsApp request signature",
			zap.Error(err),
			zap.String("requester", payload.Principal.String()),
		)
		return rpc.SignedPayload{}, multiplexer.NewErrorResponse(multiplexer.CodeUnknownError, multiplexer.Codespace, err)
	}

	if !valid {
		app.logger.Warn(
			"Got invalid EventsApp request signature",
			zap.String("requester", payload.Principal.String()),
		)
		return rpc.SignedPayload{}, multiplexer.NewErrorResponse(rpc.CodeInvalidSignature, rpc.Codespace, nil)
	}

	return payload, nil
}
