package app

import (
	"log"
	"net/http"
	"net/http/pprof"
	"runtime"
)

func startPprof() {
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	var mux = http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))

	var server = &http.Server{
		Addr:    "localhost:6060",
		Handler: mux,
	}
	log.Println("pprof listening on", server.Addr)
	log.Println(server.ListenAndServe())
}
