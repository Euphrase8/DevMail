package httpapi

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"regexp"
	"strings"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsUUID(id string) bool { return uuidPattern.MatchString(id) }
func ContractMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, param := range c.Params {
			if (param.Key == "id" || strings.HasSuffix(param.Key, "_id")) && !IsUUID(param.Value) {
				Fail(c, 422, ErrValidation, "Invalid resource ID.")
				return
			}
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		c.Next()
	}
}
