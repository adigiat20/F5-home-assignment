// Command redactor reads NDJSON HTTP transactions and redacts sensitive data
// according to a YAML rules file. See README.md.
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
	flag.Parse()

	cfg, err := LoadConfig(*rulesPath)
	if err != nil {
		fatal(err)
	}
	red := NewRedactor(cfg)

	in := io.Reader(os.Stdin)
	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fatal(err)
		}
		defer f.Close()
		in = f
	}

	if err := run(red, in, os.Stdout); err != nil {
		fatal(err)
	}
	red.WriteSummary(os.Stderr) // stdout = data only, stderr = summary
}

// run streams the input line by line through the redactor.
func run(red *Redactor, in io.Reader, out io.Writer) error {
	w := bufio.NewWriter(out)
	rd := bufio.NewReaderSize(in, 1<<20)
	for {
		line, err := rd.ReadBytes('\n') // no 64KB line limit, unlike bufio.Scanner
		if len(bytes.TrimSpace(line)) > 0 {
			w.WriteString(red.ProcessLine(bytes.TrimRight(line, "\r\n")))
			w.WriteByte('\n')
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return w.Flush()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}