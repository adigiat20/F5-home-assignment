package configs

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"strings"
)

// apply performs one of: mask | partial | hash | remove on the string s.
func (r *Redactor) apply(action, s string) string {
	st := r.cfg.Settings
	switch action {
	case "mask":
		return st.Mask.Replacement
	case "partial":
		rs := []rune(s)
		keep := st.Partial.KeepLast
		if keep >= len(rs) {
			keep = 0 // too short: hide everything rather than leak it
		}
		return strings.Repeat(st.Partial.Fill, len(rs)-keep) + string(rs[len(rs)-keep:])
	case "hash":
		if st.Hash.Algorithm == "sha512" {
			sum := sha512.Sum512([]byte(s))
			return hex.EncodeToString(sum[:])
		}
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	return ""
}
// luhn reports whether s (digits, optionally with spaces/dashes) passes the
// Luhn checksum and has a plausible card length (13-19 digits).
func luhn(s string) bool {
	sum, n, double := 0, 0, false
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c == ' ' || c == '-' {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		d := int(c - '0')
		if double {
			if d *= 2; d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
		n++
	}
	return n >= 13 && n <= 19 && sum%10 == 0
}