package main

import (
	"sort"
	"strings"
)

type hit struct {
	start, end int
	rule       *ValueRule
	order      int // index of the rule in the config
}

// redactText applies all value rules to plain text. Matches are found on the
// ORIGINAL string, overlaps are resolved (earliest start wins, then rule order),
// and the string is rebuilt once. Rules therefore never see each other's output.
func (r *Redactor) redactText(s string) string {
	var hits []hit
	for i := range r.cfg.ValueRules {
		vr := &r.cfg.ValueRules[i]
		for _, loc := range vr.re.FindAllStringIndex(s, -1) {
			if vr.Validator == "luhn" && !luhn(s[loc[0]:loc[1]]) {
				continue
			}
			hits = append(hits, hit{loc[0], loc[1], vr, i})
		}
	}
	if len(hits) == 0 {
		return s
	}
	sort.Slice(hits, func(a, b int) bool {
		if hits[a].start != hits[b].start {
			return hits[a].start < hits[b].start
		}
		return hits[a].order < hits[b].order
	})

	var b strings.Builder
	last := 0
	for _, h := range hits {
		if h.start < last { // overlaps an already-redacted span
			continue
		}
		b.WriteString(s[last:h.start])
		b.WriteString(r.apply(h.rule.Action, s[h.start:h.end]))
		last = h.end
		r.counts[h.rule.Name]++
	}
	b.WriteString(s[last:])
	return b.String()
}