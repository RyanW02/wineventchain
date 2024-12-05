package server

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	types "github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/blockchain"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport/payload"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
)

func (s *Server[T, U]) HandleSubmit(c *gin.Context) {
	var req types.SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.StoreEvent(c, req); err != nil {
		c.JSON(err.ResponseCode, gin.H{"error": err.Error()})
		return
	}

	marshalled, err := payload.NewPayloadMarshalled(payload.TypeBroadcastEvent, req)
	if err != nil {
		s.logger.Error("failed to create broadcast payload", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create broadcast payload"})
		return
	}

	if err := s.transport.Broadcast(marshalled); err != nil {
		s.logger.Error("failed to send submit request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send submit request"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{})
}

func (s *Server[T, U]) StoreEvent(ctx context.Context, req types.SubmitRequest) *HttpError {
	// Get principal
	principal, err := s.blockchain.GetIdentity(identity.Principal(req.Principal))
	if err != nil {
		if errors.Is(err, blockchain.ErrPrincipalNotFound) {
			return NewHttpError(http.StatusUnauthorized, "principal not found")
		} else {
			s.logger.Error("failed to get identity", zap.Error(err))
			return NewHttpError(http.StatusInternalServerError, "failed to get identity")
		}
	}

	// Validate signature
	signature, err := hex.DecodeString(req.Signature)
	if err != nil {
		return NewHttpError(http.StatusBadRequest, "invalid signature")
	}

	hash := req.EventData.Hash()
	signatureValid := ed25519.Verify(principal.PublicKey, hash[:], signature)
	if !signatureValid {
		s.logger.Warn(
			"got invalid event data signature",
			zap.String("principal", req.Principal),
			zap.Stringer("event_id", req.EventId),
			zap.Stringer("tx_hash", req.TxHash),
			zap.String("signature", req.Signature),
		)
		return NewHttpError(http.StatusForbidden, "signature is invalid")
	}

	// Fetch event data from blockchain
	event, err := s.blockchain.GetEventByTx(req.TxHash)
	if err != nil {
		if errors.Is(err, blockchain.ErrEventNotFound) {
			return NewHttpError(http.StatusNotFound, "event not found")
		}

		s.logger.Error("failed to get event by tx", zap.Error(err))
		return NewHttpError(http.StatusInternalServerError, "failed to get event by tx")
	}

	// Check we are talking about the same event
	if !bytes.Equal(event.Metadata.EventId, req.EventId) {
		s.logger.Warn(
			"event id does not match",
			zap.Stringer("tx_hash", req.TxHash),
			zap.Stringer("event_id", event.Metadata.EventId),
			zap.Stringer("request_event_id", req.EventId),
		)
		return NewHttpError(http.StatusBadRequest, "event id does not match")
	}

	// Check the same principal that submitted the event is submitting the data. This is enforced by the signature check.
	if event.Metadata.Principal.String() != req.Principal {
		s.logger.Warn("got event data submitted by different principal",
			zap.Stringer("tx_hash", req.TxHash),
			zap.Stringer("event_id", event.Metadata.EventId),
			zap.String("on_chain_principal", event.Metadata.Principal.String()),
			zap.String("submitted_principal", req.Principal),
		)
		return NewHttpError(http.StatusForbidden, "principal does not match")
	}

	// Check that the on-chain hash matches the hash of the event data submitted
	if event.OffChainHash != hex.EncodeToString(hash) {
		s.logger.Warn(
			"event data does not match the on-chain hash",
			zap.Stringer("tx_hash", req.TxHash),
			zap.Stringer("event_id", event.Metadata.EventId),
			zap.String("on_chain_hash", event.OffChainHash),
			zap.String("submitted_hash", hex.EncodeToString(hash)),
		)
		return NewHttpError(http.StatusBadRequest, "event data does not match")
	}

	fullEvent := events.StoredEvent{
		EventWithData: events.EventWithData{
			Event:     event.Event,
			EventData: req.EventData,
		},
		Metadata: event.Metadata,
		TxHash:   req.TxHash,
	}

	if err := s.repository.Events().Store(ctx, fullEvent); err != nil {
		if errors.Is(err, repository.ErrEventAlreadyStored) {
			// Just log, don't throw error
			s.logger.Debug("Received duplicate event", zap.Stringer("event_id", req.EventId))
		} else {
			s.logger.Error("failed to store event", zap.Error(err))
			return NewHttpError(http.StatusInternalServerError, "failed to store event")
		}
	}

	// Remove missing event marker from state store
	if err := s.state.RemoveMissingEvent(ctx, event.Metadata.EventId); err != nil {
		// Log as error, but don't return HTTP response error, as no need to re-submit. The harmoniser will sort
		// out the issue.
		s.logger.Error("failed to remove missing event from state DB", zap.Error(err))
	}

	if s.eventBroadcastCh != nil {
		s.eventBroadcastCh <- fullEvent
	}

	return nil
}
