package monitoring

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"sync"
	"time"
)

type Metrics struct {
	mu      sync.Mutex
	count   map[string]int64
	seconds map[string]float64
}

func NewMetrics() *Metrics { return &Metrics{count: map[string]int64{}, seconds: map[string]float64{}} }
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		key := fmt.Sprintf(`method=%q,route=%q,status=%q`, c.Request.Method, route, fmt.Sprint(c.Writer.Status()))
		m.mu.Lock()
		m.count[key]++
		m.seconds[key] += time.Since(start).Seconds()
		m.mu.Unlock()
	}
}
func (m *Metrics) Serve(c *gin.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c.Header("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintln(c.Writer, "# TYPE devmail_http_requests_total counter\n# TYPE devmail_http_request_duration_seconds_sum counter")
	for key, count := range m.count {
		fmt.Fprintf(c.Writer, "devmail_http_requests_total{%s} %d\ndevmail_http_request_duration_seconds_sum{%s} %f\n", key, count, key, m.seconds[key])
	}
}
