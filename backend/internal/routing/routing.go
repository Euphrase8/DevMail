package routing

import (
	"errors"
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/mail"
)

func Validate(m map[string]any) error {
	for _, key := range []string{"source", "destination"} {
		v, err := httpapi.RequiredString(m, key)
		if err != nil {
			return err
		}
		addr, err := mail.ParseAddress(v)
		if err != nil || addr.Address != v {
			return errors.New(key + " must be an email address")
		}
	}
	if m["source"] == m["destination"] {
		return errors.New("source and destination must differ")
	}
	return nil
}
func Register(v1 *gin.RouterGroup, pool *pgxpool.Pool, middleware ...gin.HandlerFunc) {
	r := &httpapi.Resource{Pool: pool, Table: "routing_rules", Validate: Validate}
	r.Register(v1, "/routing-rules", middleware...)
}
