package docs

import (
	_ "embed"
	"net/http"

	"github.com/KrainovSD/go-packages/app"
)

//go:embed swagger.html
var swaggerHTML []byte

//go:embed swagger.json
var swaggerSpec []byte

func returnSwaggerHTML(w http.ResponseWriter, r *http.Request) {
	w.Write(swaggerHTML)
}
func returnSwaggerSpec(w http.ResponseWriter, r *http.Request) {
	w.Write(swaggerSpec)
}

func Register(m *app.Mux, sm *app.Mux) {
	m.Handle("GET /api/docs", http.HandlerFunc(returnSwaggerHTML))
	m.Handle("GET /api/docs/", http.HandlerFunc(returnSwaggerHTML))
	sm.Handle("/openapi.json", http.HandlerFunc(returnSwaggerSpec))
}
