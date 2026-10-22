package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"redactor/configs"
)

func serve(red *configs.Redactor, addr string) error {
	mux := http.NewServeMux() // get http req 

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("/redact", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_ = http.NewResponseController(w).EnableFullDuplex()
		w.Header().Set("Content-Type", "application/x-ndjson")
		if err := run(red, r.Body, w, os.Stderr); err != nil {
			log.Printf("POST /redact: %v", err)
		}
	})

	log.Printf("listening on %s", addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return srv.ListenAndServe()
}