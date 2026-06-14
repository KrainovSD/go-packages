package router

import (
	"fmt"
	"net/http"

	"github.com/KrainovSD/go-packages/internal/docs"
	"github.com/KrainovSD/go-packages/internal/internal/status"
	"github.com/KrainovSD/go-packages/internal/internal/tests"
	"github.com/KrainovSD/go-packages/internal/modules/cradle"
)

type RoutesOptions struct {
	SM     *http.ServeMux
	M      *http.ServeMux
	Cradle *cradle.Cradle
}

func InitRoutes(o *RoutesOptions) error {
	var err error
	var testsService *tests.TestsService
	if testsService, err = tests.CreateService(&tests.TestsServiceOptions{
		Cradle: o.Cradle,
	}); err != nil {
		return fmt.Errorf("create c2m service: %w", err)
	}
	/** other router */
	docs.Register(o.M, o.SM)
	if err = tests.Register(tests.TestsControllerOptions{
		M:  o.M,
		SM: o.SM,
		S:  testsService,
	}); err != nil {
		return fmt.Errorf("register c2m: %w", err)
	}
	status.Register(status.StatusControllerOptions{M: o.M, Cradle: o.Cradle})
	return nil
}
