package viewer

import (
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"
	"strings"
)

const keyPrincipal = "principal"

func (s *Server) authenticate(c *gin.Context) {
	header := c.GetHeader("Authorization")
	if header == "" {
		c.AbortWithStatusJSON(401, gin.H{"error": "missing authorization header"})
		return
	}

	token, valid := s.validateToken([]byte(header))
	if !valid {
		c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
		return
	}

	c.Set(keyPrincipal, token.Subject())

	c.Next()
}

func (s *Server) validateToken(bytes []byte) (jwt.Token, bool) {
	var parser jwt.ParseOption
	switch strings.ToUpper(s.config.ViewerServer.JWTAlgorithm) {
	case jwa.HS256.String():
		parser = jwt.WithVerify(jwa.HS256, []byte(s.config.ViewerServer.JWTSecret))
	default:
		return nil, false
	}

	token, err := jwt.Parse(bytes, parser)
	if err != nil {
		return nil, false
	}

	return token, true
}
