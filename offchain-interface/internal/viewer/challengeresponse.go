package viewer

import (
	"context"
	"crypto/ed25519"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type challengeResponseRequestBody struct {
	Principal identity.Principal `json:"principal"`
	Challenge []byte             `json:"challenge"`
	Response  []byte             `json:"response"`
}

func (s *Server) challengeResponseHandler(c *gin.Context) {
	var body challengeResponseRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	identity, err := s.blockchainClient.GetIdentity(body.Principal)
	if err != nil {
		s.logger.Error("failed to retrieve identity", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve identity"})
		return
	}

	if !identity.IsAdmin() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "principal is not an admin"})
		return
	}

	// Validate signature
	if !ed25519.Verify(identity.PublicKey, body.Challenge, body.Response) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid private key"})
		return
	}

	ctx, cancel := context.WithTimeout(c, time.Second*5)
	defer cancel()

	valid, err := s.repository.Challenges().GetAndRemoveChallenge(ctx, body.Principal, body.Challenge, s.config.ViewerServer.ChallengeLifetime.Duration())
	if err != nil {
		s.logger.Error("failed to get and remove challenge token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get and remove challengeHandler"})
		return
	}

	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid challenge response"})
		return
	}

	// Issue JWT
	token, err := jwt.NewBuilder().
		Subject(body.Principal.String()).
		IssuedAt(time.Now()).
		Build()

	if err != nil {
		s.logger.Error("failed to issue token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
		return
	}

	var signedJwt []byte
	switch s.config.ViewerServer.JWTAlgorithm {
	case jwa.HS256.String():
		signed, err := jwt.Sign(token, jwa.HS256, []byte(s.config.ViewerServer.JWTSecret))
		if err != nil {
			s.logger.Error("failed to sign token", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
			return
		}

		signedJwt = signed
	default:
		s.logger.Error("unsupported jwt algorithm in config", zap.String("algorithm", s.config.ViewerServer.JWTAlgorithm))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unsupported jwt algorithm"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": string(signedJwt),
	})
}
