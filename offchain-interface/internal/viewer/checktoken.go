package viewer

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (s *Server) checkTokenHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"principal": c.GetString(keyPrincipal),
	})
}
