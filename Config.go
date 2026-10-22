package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Settings struct {
	Mask struct {
		Replacement string `yaml:"replacement"`
	} `yaml:"mask"`
	Partial struct {
		KeepLast int    `yaml:"keep_last"`
		Fill     string `yaml:"fill"`
	} `yaml:"partial"`
	Hash struct {
		Algorithm string `yaml:"algorithm"`
	} `yaml:"hash"`
}

type KeyRule struct {
	Name  string `yaml:"name"`
	Scope string `yaml:"scope"` // headers | body | any
	Match struct {
		Exact []string `yaml:"exact"`
		Regex string   `yaml:"regex"`
	} `yaml:"match"`
	Action string `yaml:"action"`

	re    *regexp.Regexp  // compiled Match.Regex
	exact map[string]bool // lowercased Match.Exact
}

type ValueRule struct {
	Name      string `yaml:"name"`
	Pattern   string `yaml:"pattern"`
	Action    string `yaml:"action"`
	Validator string `yaml:"validator"`

	re *regexp.Regexp
}

type Config struct {
	Settings   Settings    `yaml:"settings"`
	KeyRules   []KeyRule   `yaml:"key_rules"`
	ValueRules []ValueRule `yaml:"value_rules"`
}

// LoadConfig reads, validates and pre-compiles the rules file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	// defaults survive when the YAML omits the settings block
	cfg.Settings.Mask.Replacement = "****"
	cfg.Settings.Partial.KeepLast = 4
	cfg.Settings.Partial.Fill = "*"
	cfg.Settings.Hash.Algorithm = "sha256"

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func validAction(a string) bool {
	switch a {
	case "mask", "partial", "hash", "remove":
		return true
	}
	return false
}

func (c *Config) validate() error {
	if a := c.Settings.Hash.Algorithm; a != "sha256" && a != "sha512" {
		return fmt.Errorf("unsupported hash algorithm %q", a)
	}
	if c.Settings.Partial.Fill == "" {
		c.Settings.Partial.Fill = "*"
	}

	var err error
	for i := range c.KeyRules {
		kr := &c.KeyRules[i]
		if !validAction(kr.Action) {
			return fmt.Errorf("key rule %q: bad action %q", kr.Name, kr.Action)
		}
		switch kr.Scope {
		case "", "any":
			kr.Scope = "any"
		case "headers", "body":
		default:
			return fmt.Errorf("key rule %q: bad scope %q", kr.Name, kr.Scope)
		}
		if len(kr.Match.Exact) == 0 && kr.Match.Regex == "" {
			return fmt.Errorf("key rule %q: needs match.exact or match.regex", kr.Name)
		}
		kr.exact = map[string]bool{}
		for _, n := range kr.Match.Exact {
			kr.exact[strings.ToLower(n)] = true
		}
		if kr.Match.Regex != "" {
			if kr.re, err = regexp.Compile(kr.Match.Regex); err != nil {
				return fmt.Errorf("key rule %q: %w", kr.Name, err)
			}
		}
	}
	for i := range c.ValueRules {
		vr := &c.ValueRules[i]
		if !validAction(vr.Action) {
			return fmt.Errorf("value rule %q: bad action %q", vr.Name, vr.Action)
		}
		if vr.Validator != "" && vr.Validator != "luhn" {
			return fmt.Errorf("value rule %q: unknown validator %q", vr.Name, vr.Validator)
		}
		if vr.re, err = regexp.Compile(vr.Pattern); err != nil {
			return fmt.Errorf("value rule %q: %w", vr.Name, err)
		}
	}
	return nil
}