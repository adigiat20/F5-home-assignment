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
	"runtime"
	"sync"

	"redactor/configs"
)

func main() {
	rulesPath := flag.String("rules", "rules.yaml", "path to rules file")
	serveMode := flag.Bool("serve", false, "run as an HTTP service (POST /redact, GET /healthz)")
	addr := flag.String("addr", ":8080", "listen address in --serve mode")
	workers := flag.Int("workers", runtime.NumCPU(), "number of concurrent worker goroutines")
	flag.Parse()

	// Fail closed: without valid rules we exit before reading or listening
	cfg, err := configs.LoadConfig(*rulesPath)
	if err != nil {
		fatal(err)
	}
	red := configs.NewRedactor(cfg)

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
	if err := runConcurrent(red, in, os.Stdout, os.Stderr, *workers); err != nil {
		fatal(err)
	}
	red.WriteSummary(os.Stderr) // stdout = data only, stderr = summary
}

// run is the entrypoint used by tests and server, defaulting to runtime.NumCPU workers.
func run(red *configs.Redactor, in io.Reader, out, warn io.Writer) error {
	return runConcurrent(red, in, out, warn, runtime.NumCPU())
}

// job holds the data for a single line to be processed.
type job struct {
	lineNo  int
	raw     []byte
	resChan chan result
}

// result holds the outcome of processing one line.
type result struct {
	lineNo int
	out    string
	ok     bool
}

// runConcurrent streams input through a concurrent worker pool while strictly
// preserving original line order in the output.
func runConcurrent(red *configs.Redactor, in io.Reader, out, warn io.Writer, numWorkers int) error {
	if numWorkers < 1 {
		numWorkers = 1
	}

	// Channel buffer size ensures flow control
	bufferSize := numWorkers * 8
	jobs := make(chan job, bufferSize)
	ordered := make(chan chan result, bufferSize)

	// 1. Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				outStr, ok := red.ProcessLine(j.raw)
				j.resChan <- result{
					lineNo: j.lineNo,
					out:    outStr,
					ok:     ok,
				}
			}
		}()
	}

	// 2. Writer goroutine: drains the ordered channel to write results sequentially
	type writeOutcome struct {
		err error
	}
	writerDone := make(chan writeOutcome, 1)

	go func() {
		w := bufio.NewWriter(out)
		for resChan := range ordered {
			res := <-resChan
			if res.ok {
				if _, err := w.WriteString(res.out); err != nil {
					writerDone <- writeOutcome{err: err}
					return
				}
				if err := w.WriteByte('\n'); err != nil {
					writerDone <- writeOutcome{err: err}
					return
				}
			} else {
				fmt.Fprintf(warn, "warning: line %d is not valid JSON, dropped\n", res.lineNo)
			}
		}
		writerDone <- writeOutcome{err: w.Flush()}
	}()

	// 3. Reader: reads lines and dispatches them to workers
	rd := bufio.NewReaderSize(in, 1<<20)
	lineNo := 0
	var readErr error

	for {
		line, err := rd.ReadBytes('\n')
		lineNo++
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			// Copy line slice because rd.ReadBytes reuses internal buffer
			rawCopy := make([]byte, len(bytes.TrimRight(line, "\r\n")))
			copy(rawCopy, bytes.TrimRight(line, "\r\n"))

			resCh := make(chan result, 1)
			ordered <- resCh
			jobs <- job{
				lineNo:  lineNo,
				raw:     rawCopy,
				resChan: resCh,
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			readErr = err
			break
		}
	}

	// Close jobs and wait for all workers to finish
	close(jobs)
	wg.Wait()

	// Close ordered channel so writer knows no more lines are coming
	close(ordered)

	// Wait for writer to finish flushing
	outcome := <-writerDone
	if readErr != nil {
		return readErr
	}
	return outcome.err
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
