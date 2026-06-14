package docs

import (
	_ "embed"
	"net/http"
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

func Register(m *http.ServeMux, sm *http.ServeMux) {
	m.HandleFunc("GET /api/docs", returnSwaggerHTML)
	m.HandleFunc("GET /api/docs/", returnSwaggerHTML)
	sm.HandleFunc("/openapi.json", returnSwaggerSpec)
}
