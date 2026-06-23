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

func Register(o TestsControllerOptions) error {
	var c = testsController{
		s: o.S,
	}
	o.M.HandleFunc("GET /api/v1/tests", c.Test)
	o.M.HandleFunc("GET /api/v2/tests", c.Test2)
	return nil
}

// @Summary Test
// @Description Test
// @Tags Tests
// @Success 200 {string} string
// @Router /api/v2/tests [get]
func (c *testsController) Test2(w http.ResponseWriter, req *http.Request) {
	// var test = make([]string, 0, 0)
	// fmt.Println(test[1])
	// w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
	// fmt.Fprint(w, "ok")
	w.WriteHeader(http.StatusNotFound)
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
