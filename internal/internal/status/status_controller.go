package status

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/KrainovSD/go-packages/app"
	"github.com/KrainovSD/go-packages/internal/modules/cradle"
)

type StatusControllerOptions struct {
	M      *app.Mux
	Cradle *cradle.Cradle
}

type statusController struct {
	m *app.Mux
}

type Health struct {
	Ok bool `json:"ok"`
}

func Register(o StatusControllerOptions) {
	var c = statusController{
		m: o.M,
	}
	o.M.Handle("GET /api/ping", http.HandlerFunc(c.Ping))
	o.M.Handle("GET /api/healthz", http.HandlerFunc(c.Healthcheck))
	o.M.Handle("GET /api/metrics", o.Cradle.Metrics.Handle())
}

// @Summary Ping
// @Description Ping
// @Tags Status
// @Success 200 {string} string
// @Router /api/ping [get]
func (c *statusController) Ping(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
	fmt.Fprint(w, "pong")
}

// @Summary Healthcheck
// @Description Healthcheck
// @Tags Status
// @Success 200 {object} Health
// @Router /api/healthz [get]
func (c *statusController) Healthcheck(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Health{Ok: true})
}

// @Summary Metrics
// @Description Metrics
// @Tags Status
// @Success 200 {string} string
// @Router /api/metrics [get]
func (c *statusController) Metrics(w http.ResponseWriter, req *http.Request) {

}
