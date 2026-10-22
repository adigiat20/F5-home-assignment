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
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP redacted_bytes_total Total bytes processed\n")
		fmt.Fprintf(w, "# TYPE redacted_bytes_total counter\n")
		fmt.Fprintf(w, "redacted_bytes_total %d\n", red.GetBytesProcessed())
		
		fmt.Fprintf(w, "# HELP redactions_total Total redaction events\n")
		fmt.Fprintf(w, "# TYPE redactions_total counter\n")
		fmt.Fprintf(w, "redactions_total %d\n", red.GetRedactionCount())
	})

	log.Printf("listening on %s", addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return srv.ListenAndServe()
}