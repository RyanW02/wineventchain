package viewer

import (
	"context"
	"crypto/rand"
	"errors"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/blockchain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type challengeRequestBody struct {
	Principal identity.Principal `json:"principal"`
}

type challengeResponseBody struct {
	Challenge []byte `json:"challenge"`
}

func (s *Server) challengeHandler(c *gin.Context) {
	var body challengeRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	identity, err := s.blockchainClient.GetIdentity(body.Principal)
	if err != nil {
		if errors.Is(err, blockchain.ErrPrincipalNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "principal not found"})
			return
		} else {
			s.logger.Error("failed to retrieve identity", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve identity"})
			return
		}
	}

	if !identity.IsAdmin() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "principal is not an admin"})
		return
	}

	ctx, cancel := context.WithTimeout(c, time.Second*5)
	defer cancel()

	challenge, err := generateChallenge(256)
	if err != nil {
		s.logger.Error("failed to generate challenge", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate challenge"})
		return
	}

	if err := s.repository.Challenges().AddChallenge(ctx, body.Principal, challenge); err != nil {
		s.logger.Error("failed to add challengeHandler", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add challenge"})
		return
	}

	c.JSON(http.StatusOK, challengeResponseBody{
		Challenge: challenge,
	})
}

func generateChallenge(size int) ([]byte, error) {
	challenge := make([]byte, size)
	if _, err := rand.Read(challenge); err != nil {
		return nil, err
	}

	return challenge, nil
}
