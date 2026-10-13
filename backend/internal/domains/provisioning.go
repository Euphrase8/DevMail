package domains

import (
	"github.com/Euphrase8/DevMail/backend/internal/httpapi"
	"github.com/gin-gonic/gin"
)

func (h *Handler) Provision(c *gin.Context) {
	d, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpapi.Fail(c, 404, httpapi.ErrNotFound, "Domain not found.")
		return
	}
	if h.svc.enqueue == nil {
		httpapi.Respond(c, 200, gin.H{"status": "unmanaged"})
		return
	}
	if d.VerifiedAt == "" {
		httpapi.Fail(c, 409, httpapi.ErrConflict, "Verify domain ownership before provisioning.")
		return
	}
	if err := h.svc.enqueue(c.Request.Context(), "domain.provision", map[string]string{"domainId": d.ID}); err != nil {
		httpapi.InternalError(c)
		return
	}
	httpapi.Respond(c, 202, gin.H{"status": "queued"})
}
