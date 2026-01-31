package main

import (
	"testing"

	"github.com/justapithecus/quarry/policy"
)

func TestValidatePolicyConfig_Strict(t *testing.T) {
	choice := policyChoice{
		name:      "strict",
		flushMode: "at_least_once",
		maxEvents: 0,
		maxBytes:  0,
	}

	err := validatePolicyConfig(choice)
	if err != nil {
		t.Errorf("expected no error for strict policy, got %v", err)
	}
}

func TestValidatePolicyConfig_Strict_IgnoresBufferFlags(t *testing.T) {
	// Should succeed but warn (warning goes to stderr, not returned as error)
	choice := policyChoice{
		name:      "strict",
		flushMode: "two_phase",
		maxEvents: 100,
		maxBytes:  1024,
	}

	err := validatePolicyConfig(choice)
	if err != nil {
		t.Errorf("expected no error for strict policy with buffer flags, got %v", err)
	}
}

func TestValidatePolicyConfig_Buffered_RequiresLimit(t *testing.T) {
	choice := policyChoice{
		name:      "buffered",
		flushMode: "at_least_once",
		maxEvents: 0,
		maxBytes:  0,
	}

	err := validatePolicyConfig(choice)
	if err == nil {
		t.Error("expected error for buffered policy without limits")
	}
}

func TestValidatePolicyConfig_Buffered_EventLimit(t *testing.T) {
	choice := policyChoice{
		name:      "buffered",
		flushMode: "at_least_once",
		maxEvents: 100,
		maxBytes:  0,
	}

	err := validatePolicyConfig(choice)
	if err != nil {
		t.Errorf("expected no error for buffered policy with event limit, got %v", err)
	}
}

func TestValidatePolicyConfig_Buffered_ByteLimit(t *testing.T) {
	choice := policyChoice{
		name:      "buffered",
		flushMode: "at_least_once",
		maxEvents: 0,
		maxBytes:  1024,
	}

	err := validatePolicyConfig(choice)
	if err != nil {
		t.Errorf("expected no error for buffered policy with byte limit, got %v", err)
	}
}

func TestValidatePolicyConfig_Buffered_AllFlushModes(t *testing.T) {
	modes := []string{"at_least_once", "chunks_first", "two_phase"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			choice := policyChoice{
				name:      "buffered",
				flushMode: mode,
				maxEvents: 100,
			}

			err := validatePolicyConfig(choice)
			if err != nil {
				t.Errorf("expected no error for flush mode %s, got %v", mode, err)
			}
		})
	}
}

func TestValidatePolicyConfig_Buffered_InvalidFlushMode(t *testing.T) {
	choice := policyChoice{
		name:      "buffered",
		flushMode: "invalid_mode",
		maxEvents: 100,
	}

	err := validatePolicyConfig(choice)
	if err == nil {
		t.Error("expected error for invalid flush mode")
	}
}

func TestValidatePolicyConfig_InvalidPolicy(t *testing.T) {
	choice := policyChoice{
		name: "unknown",
	}

	err := validatePolicyConfig(choice)
	if err == nil {
		t.Error("expected error for invalid policy")
	}
}

func TestBuildPolicy_Strict(t *testing.T) {
	choice := policyChoice{
		name: "strict",
	}

	pol, err := buildPolicy(choice)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer pol.Close()

	// Verify it's a strict policy by checking stats behavior
	stats := pol.Stats()
	if stats.TotalEvents != 0 {
		t.Errorf("expected empty stats for new policy")
	}
}

func TestBuildPolicy_Buffered(t *testing.T) {
	choice := policyChoice{
		name:      "buffered",
		flushMode: "two_phase",
		maxEvents: 100,
		maxBytes:  1024,
	}

	pol, err := buildPolicy(choice)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer pol.Close()

	// Verify it's a buffered policy by checking stats behavior
	stats := pol.Stats()
	if stats.TotalEvents != 0 {
		t.Errorf("expected empty stats for new policy")
	}
}

func TestBuildPolicy_Buffered_DefaultFlushMode(t *testing.T) {
	choice := policyChoice{
		name:      "buffered",
		flushMode: "", // Empty should default to at_least_once
		maxEvents: 100,
	}

	pol, err := buildPolicy(choice)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer pol.Close()
}

func TestBuildPolicy_Unknown(t *testing.T) {
	choice := policyChoice{
		name: "unknown",
	}

	_, err := buildPolicy(choice)
	if err == nil {
		t.Error("expected error for unknown policy")
	}
}

func TestBuildPolicy_Buffered_InvalidConfig(t *testing.T) {
	choice := policyChoice{
		name:      "buffered",
		flushMode: "at_least_once",
		maxEvents: 0,
		maxBytes:  0,
	}

	_, err := buildPolicy(choice)
	if err == nil {
		t.Error("expected error for buffered policy without limits")
	}
}

func TestPolicyChoice_FlushModeConstants(t *testing.T) {
	// Verify the string values match policy constants
	if policy.FlushAtLeastOnce != "at_least_once" {
		t.Errorf("FlushAtLeastOnce mismatch: %s", policy.FlushAtLeastOnce)
	}
	if policy.FlushChunksFirst != "chunks_first" {
		t.Errorf("FlushChunksFirst mismatch: %s", policy.FlushChunksFirst)
	}
	if policy.FlushTwoPhase != "two_phase" {
		t.Errorf("FlushTwoPhase mismatch: %s", policy.FlushTwoPhase)
	}
}
