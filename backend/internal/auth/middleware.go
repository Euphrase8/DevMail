package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
)

const userKey = "dm.user"

// CurrentUser returns the authenticated user attached by RequireAuth, or nil
// when the caller is anonymous.
func CurrentUser(c *gin.Context) *User {
	if v, ok := c.Get(userKey); ok {
		if u, ok := v.(*User); ok {
			return u
		}
	}
	return nil
}

// RequireAuth rejects anonymous requests and resolves the user from the
// session cookie.
func RequireAuth(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(SessionCookie)
		if err != nil || token == "" {
			httpapi.Fail(c, http.StatusUnauthorized, httpapi.ErrUnauthorized, "Authentication required.")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		user, err := svc.UserBySession(ctx, token)
		if err != nil {
			httpapi.Fail(c, http.StatusUnauthorized, httpapi.ErrUnauthorized, "Session is invalid or expired.")
			return
		}
		c.Set(userKey, user)
		c.Next()
	}
}

// RequireRole enforces a minimum role before calling the handler.
func RequireRole(min Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			httpapi.Fail(c, http.StatusUnauthorized, httpapi.ErrUnauthorized, "Authentication required.")
			return
		}
		if !u.Role.Allowed(min) {
			httpapi.Fail(c, http.StatusForbidden, httpapi.ErrForbidden, "You do not have permission to perform this action.")
			return
		}
		c.Next()
	}
}