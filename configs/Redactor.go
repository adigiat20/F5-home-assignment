package configs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

type Redactor struct {
	cfg    *Config
	mu     sync.Mutex
	counts map[string]int
}

func (r *Redactor) inc(name string) {
	r.mu.Lock()
	r.counts[name]++
	r.mu.Unlock()
}

func NewRedactor(cfg *Config) *Redactor {
	return &Redactor{cfg: cfg, counts: map[string]int{}}
}

// ProcessLine redacts one NDJSON line. ok=false means the line is not valid
// JSON. It cannot be redacted reliably (e.g. a truncated token slips past the
// regexes), so nothing is returned and the caller must not emit it.
func (r *Redactor) ProcessLine(line []byte) (out string, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber() // keep numbers exactly as written
	var v any
	if !json.Valid(line) || dec.Decode(&v) != nil {
		return "", false
	}
	return marshal(r.walk(v, "record")), true
}

// WriteSummary prints redaction counts per rule, sorted by rule name.
func (r *Redactor) WriteSummary(w io.Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.counts))
	for n := range r.counts {
		names = append(names, n)
	}
	sort.Strings(names)
		fmt.Fprintln(w, "redaction summary:")
	for _, n := range names {
		fmt.Fprintf(w, "%-20s %d\n", n, r.counts[n])
	}
}

// Scopes: "record" = top of a line or the response object;
// "headers" and "body" as in the config; "meta" = ts, method, path, status...

// walk recursively redacts any decoded JSON value.
func (r *Redactor) walk(v any, scope string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if rule := r.keyRule(k, scope); rule != nil {
				r.counts[rule.Name]++
				if rule.Action == "remove" {
					delete(x, k)
					continue
				}
				if s, ok := val.(string); ok {
					x[k] = r.apply(rule.Action, s)
				} else {
					x[k] = r.apply(rule.Action, marshal(val)) // number, object, null...
				}
				continue
			}
			x[k] = r.walk(val, childScope(k, scope))
		}
		return x
	case []any:
		for i := range x {
			x[i] = r.walk(x[i], scope)
		}
		return x
	case string:
		return r.redactString(x, scope)
	}
	return v // numbers, bools, null
}

// keyRule returns the first key rule matching this key in this scope, or nil.
func (r *Redactor) keyRule(key, scope string) *KeyRule {
	if scope == "record" {
		scope = "meta"
	}
	lk := strings.ToLower(key)
	for i := range r.cfg.KeyRules {
		kr := &r.cfg.KeyRules[i]
		if kr.Scope != "any" && kr.Scope != scope {
			continue
		}
		if kr.exact[lk] || (kr.re != nil && kr.re.MatchString(key)) {
			return kr
		}
	}
	return nil
}

// childScope decides which scope the value under key belongs to.
func childScope(key, scope string) string {
	if scope != "record" {
		return scope // headers/body/meta are inherited
	}
	switch key {
	case "headers":
		return "headers"
	case "body":
		return "body"
	case "response":
		return "record"
	}
	return "meta"
}

func (r *Redactor) redactString(s, scope string) string {
	t := strings.TrimSpace(s)
	if len(t) > 1 && (t[0] == '{' || t[0] == '[') {
		dec := json.NewDecoder(strings.NewReader(t))
		dec.UseNumber()
		var inner any
		if dec.Decode(&inner) == nil && !dec.More() {
			return marshal(r.walk(inner, scope))
		}
	}
	return r.redactText(s)
}

// marshal encodes v as compact JSON without HTML escaping (&, <, > stay as-is).
func marshal(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimRight(buf.String(), "\n")
}