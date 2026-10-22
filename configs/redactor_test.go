
package configs

import (
	"encoding/json"
	"strings"
	"testing"
	"os"
)

func testRedactor(t *testing.T, rules string) *Redactor {
	t.Helper()

	path := t.TempDir() + "/rules.yaml"
	if err := os.WriteFile(path, []byte(rules), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	return NewRedactor(cfg)
}

func TestMaskNestedKey(t *testing.T) {
	r := testRedactor(t, `
key_rules:
  - name: password
    scope: any
    match:
      exact: [password]
    action: mask
`)

	got, ok := r.ProcessLine([]byte(
		`{"user":{"password":"secret"},"name":"Alice"}`,
	))
	if !ok {
		t.Fatal("valid JSON was rejected")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatal(err)
	}

	user := result["user"].(map[string]any)
	if user["password"] != "****" {
		t.Errorf("password = %v, want ****", user["password"])
	}
	if result["name"] != "Alice" {
		t.Errorf("unrelated value changed: %v", result["name"])
	}
}

func TestRemoveKey(t *testing.T) {
	r := testRedactor(t, `
key_rules:
  - name: remove-token
    scope: any
    match:
      exact: [token]
    action: remove
`)

	got, ok := r.ProcessLine([]byte(`{"token":"secret","ok":true}`))
	if !ok {
		t.Fatal("valid JSON was rejected")
	}

	if strings.Contains(got, `"token"`) {
		t.Errorf("token key was not removed: %s", got)
	}
	if !strings.Contains(got, `"ok":true`) {
		t.Errorf("unrelated field was lost: %s", got)
	}
}

func TestInvalidJSONRejected(t *testing.T) {
	r := testRedactor(t, `
key_rules:
  - name: password
    scope: any
    match:
      exact: [password]
    action: mask
`)

	got, ok := r.ProcessLine([]byte(`{"password":`))
	if ok || got != "" {
		t.Fatalf("got (%q, %v), want empty output and false", got, ok)
	}
}

func TestEmailInsideText(t *testing.T) {
	r := testRedactor(t, `
value_rules:
  - name: email
    pattern: '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}'
    action: mask
`)

	got, ok := r.ProcessLine(
		[]byte(`{"body":"contact alice@example.com please"}`),
	)
	if !ok {
		t.Fatal("valid JSON was rejected")
	}

	if strings.Contains(got, "alice@example.com") {
		t.Errorf("email was not redacted: %s", got)
	}
	if !strings.Contains(got, "contact ") || !strings.Contains(got, " please") {
		t.Errorf("surrounding text was not preserved: %s", got)
	}
}

func TestEmbeddedJSONString(t *testing.T) {
	r := testRedactor(t, `
key_rules:
  - name: token
    scope: any
    match:
      exact: [token]
    action: mask
`)

	got, ok := r.ProcessLine(
		[]byte(`{"body":"{\"token\":\"secret\"}"}`),
	)
	if !ok {
		t.Fatal("valid JSON was rejected")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatal(err)
	}

	body := result["body"].(string)
	if strings.Contains(body, `"secret"`) {
		t.Errorf("embedded token was not redacted: %s", body)
	}
}

func TestRedactionIsIdempotent(t *testing.T) {
	r := testRedactor(t, `
key_rules:
  - name: password
    scope: any
    match:
      exact: [password]
    action: mask
`)

	first, ok := r.ProcessLine([]byte(`{"password":"secret"}`))
	if !ok {
		t.Fatal("first pass failed")
	}

	second, ok := r.ProcessLine([]byte(first))
	if !ok {
		t.Fatal("second pass failed")
	}

	if first != second {
		t.Errorf("output changed on second pass:\nfirst: %s\nsecond: %s", first, second)
	}
}
