package tests

import (
	"fmt"
	"net/http"

	"github.com/KrainovSD/go-packages/web"
)

type TestsControllerOptions struct {
	SM *http.ServeMux
	M  *http.ServeMux
	S  *TestsService
}

type testsController struct {
	s *TestsService
}

// @Summary Test
// @Description Test
// @Tags Tests
// @Success 200 {string} string
// @Router /api/v1/tests [get]
func (c *testsController) Test(w http.ResponseWriter, req *http.Request) {
	var err error

	if err = c.s.Test(req.Context()); err != nil {
		web.SendError(w, web.ErrorResponse{Error: err, Status: 409})
		return
	}
	w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
	fmt.Fprint(w, "ok")
}

func Register(o TestsControllerOptions) error {
	var c = testsController{
		s: o.S,
	}
	o.M.HandleFunc("GET /api/v1/tests", c.Test)
	return nil
}
