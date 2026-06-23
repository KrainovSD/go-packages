package web

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

type GoalkeeperMiddleware struct {
}

func CreateGoalkeeperMiddleware() *GoalkeeperMiddleware {
	return &GoalkeeperMiddleware{}
}

func (m *GoalkeeperMiddleware) Register(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				if writer, ok := w.(*ResponseWriter); ok {
					var stack = debug.Stack()
					switch e := err.(type) {
					case error:
						writer.SetPanic(e, stack)
					case string:
						writer.SetPanic(fmt.Errorf("%s", e), stack)
					default:
						writer.SetPanic(fmt.Errorf("%v", e), stack)
					}
				}
				InternalServerError(w)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
