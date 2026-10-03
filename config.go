package main

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	ErrEmptyPool       = errors.New("live_auth_ids cannot be empty after trimming and deduplication")
	ErrInvalidStrategy = errors.New("invalid strategy: must be fill-first or round-robin")
	ErrEmptyConfigYAML = errors.New("plugin configuration YAML is required")
)

// NormalizeAndValidateConfig takes raw configuration, trims strings, removes duplicates and empty entries,
// validates the strategy and pool, and applies default values.
func NormalizeAndValidateConfig(raw RawConfig) (Config, error) {
	seen := make(map[string]struct{}, len(raw.LiveAuthIDs))
	var normalizedPool []string
	for _, id := range raw.LiveAuthIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalizedPool = append(normalizedPool, trimmed)
	}

	if len(normalizedPool) == 0 {
		return Config{}, ErrEmptyPool
	}

	strategy := strings.TrimSpace(strings.ToLower(raw.Strategy))
	if strategy == "" {
		strategy = DefaultStrategy
	} else if strategy != StrategyFillFirst && strategy != StrategyRoundRobin {
		return Config{}, fmt.Errorf("%w: %q (allowed: %s, %s)", ErrInvalidStrategy, raw.Strategy, StrategyFillFirst, StrategyRoundRobin)
	}

	failClosed := DefaultFailClosed
	if raw.FailClosed != nil {
		failClosed = *raw.FailClosed
	}

	return Config{
		LiveAuthIDs: normalizedPool,
		Strategy:    strategy,
		FailClosed:  failClosed,
	}, nil
}

// ParseConfig decodes YAML into RawConfig and normalizes it.
func ParseConfig(rawYAML []byte) (Config, error) {
	if len(bytesTrimSpace(rawYAML)) == 0 {
		return Config{}, ErrEmptyConfigYAML
	}

	var raw RawConfig
	if errUnmarshal := yaml.Unmarshal(rawYAML, &raw); errUnmarshal != nil {
		return Config{}, fmt.Errorf("failed to unmarshal plugin config: %w", errUnmarshal)
	}

	return NormalizeAndValidateConfig(raw)
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
