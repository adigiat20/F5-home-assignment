package main

import (
	"reflect"
	"testing"
	"redactor/configs"
)

func TestLuhnValidator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid Visa", "4111 1111 1111 1111", true},
		{"Valid Card with hyphens", "4111-1111-1111-1111", true},
		{"Invalid Luhn Digit", "4111111111111112", false},
		{"Too Short", "12345", false},
		{"Invalid Characters", "4111a11111111111", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := luhn(tt.input)
			if got != tt.expected {
				t.Errorf("luhn(%q) = %v; want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRedactValue_EdgeCases(t *testing.T) {
	cfg := &Config{}
	cfg.Settings.Mask.Replacement = "****"
	cfg.Settings.Partial.KeepLast = 4
	cfg.Settings.Partial.Fill = "*"

	red := NewRedactor(cfg)

	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "Deeply Nested Object",
			input:    map[string]any{"a": map[string]any{"b": map[string]any{"password": "secret"}}},
			expected: map[string]any{"a": map[string]any{"b": map[string]any{"password": "****"}}},
		},
		{
			name:     "Nested Slice with Email",
			input:    []any{[]any{map[string]any{"email": "user@example.com"}}},
			expected: []any{[]any{map[string]any{"email": "fb98d44ad7501a959f3f4f4a3f004fe2d9e581ea6207e218c4b02c08a4d75adf"}}},
		},
		{
			name:     "Unicode & Emoji handling",
			input:    "שם: טנאקה 😀 test@example.com",
			expected: "שם: טנאקה 😀 973dfe463ec85785f5f95af5ba3906eedb2d931c24e69824a89ea64dba4e8d3c",
		},
		{
			name:     "Null / Nil Value",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := red.RedactValue(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("RedactValue() = %v; want %v", got, tt.expected)
			}
		})
	}
}

func TestIdempotency(t *testing.T) {
	cfg := &Config{}
	cfg.Settings.Mask.Replacement = "****"
	red := NewRedactor(cfg)

	input := map[string]any{"password": "my-secret-password"}
	
	firstPass := red.RedactValue(input)
	
	secondPass := red.RedactValue(firstPass)

	if !reflect.DeepEqual(firstPass, secondPass) {
		t.Errorf("Idempotency test failed. First: %v, Second: %v", firstPass, secondPass)
	}
}