package middlewares

import (
	"fmt"
	"net/http"

	"github.com/KrainovSD/go-packages/app"
)

type AuthOptions struct {
	Strict bool
}

func NewAuth(o AuthOptions) app.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token = r.Header.Get("Authorization")
			fmt.Println("token " + token)
			next.ServeHTTP(w, r)
		})
	}
}

type LoggerOptions struct {
	Strict bool
}

func NewLogger(o LoggerOptions) app.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("request "+r.Method+" ", r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}
