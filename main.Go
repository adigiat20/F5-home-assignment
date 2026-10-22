// Command redactor reads NDJSON HTTP transactions and redacts sensitive data
// according to a YAML rules file, as a CLI filter or an HTTP service (--serve)
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	rulesPath := flag.String("rules", "rules.yaml", "path to rules file")
	serveMode := flag.Bool("serve", false, "run as an HTTP service (POST /redact, GET /healthz)")
	addr := flag.String("addr", ":8080", "listen address in --serve mode")
	flag.Parse()

	// Fail closed: without valid rules we exit before reading or listening
	cfg, err := LoadConfig(*rulesPath)
	if err != nil {
		fatal(err)
	}
	red := NewRedactor(cfg)

	if *serveMode {
		fatal(serve(red, *addr)) // only returns on error
	}

	in := io.Reader(os.Stdin)
	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fatal(err)
		}
		defer f.Close()
		in = f
	}
	if err := run(red, in, os.Stdout, os.Stderr); err != nil {
		fatal(err)
	}
	red.WriteSummary(os.Stderr) // stdout = data only, stderr = summary
}

// run streams the input line by line through the redactor. Lines that are not
// valid JSON are dropped with a warning (fail closed).
func run(red *Redactor, in io.Reader, out, warn io.Writer) error {
	w := bufio.NewWriter(out)
	rd := bufio.NewReaderSize(in, 1<<20) // ReadBytes has no 64KB line limit, unlike bufio.Scanner
	lineNo := 0
	for {
		line, err := rd.ReadBytes('\n')
		lineNo++
		if len(bytes.TrimSpace(line)) > 0 {
			if res, ok := red.ProcessLine(bytes.TrimRight(line, "\r\n")); ok {
				w.WriteString(res)
				w.WriteByte('\n')
			} else {
				fmt.Fprintf(warn, "warning: line %d is not valid JSON, dropped\n", lineNo)
			}
		}
		if err == io.EOF {
			return w.Flush()
		}
		if err != nil {
			return err
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}