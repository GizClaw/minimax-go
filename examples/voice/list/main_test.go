package main

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeVoiceType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty defaults to all", input: "", want: "all"},
		{name: "trim and lower", input: "  SYSTEM  ", want: "system"},
		{name: "voice cloning", input: "voice_cloning", want: "voice_cloning"},
		{name: "voice generation", input: "voice_generation", want: "voice_generation"},
		{name: "all", input: "all", want: "all"},
		{name: "invalid", input: "sys", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeVoiceType(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeVoiceType() error = nil, want non-nil")
				}

				if !strings.Contains(err.Error(), "system|voice_cloning|voice_generation|all") {
					t.Fatalf("normalizeVoiceType() error = %v, want allowed values hint", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("normalizeVoiceType() error = %v, want nil", err)
			}

			if got != tc.want {
				t.Fatalf("normalizeVoiceType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOptionalEnvDuration(t *testing.T) {
	const envKey = "MINIMAX_VOICE_TIMEOUT_TEST"

	t.Run("empty value treated as not set", func(t *testing.T) {
		t.Setenv(envKey, "")
		_, set, err := optionalEnvDuration(envKey)
		if err != nil {
			t.Fatalf("optionalEnvDuration() error = %v, want nil", err)
		}

		if set {
			t.Fatal("optionalEnvDuration() set = true, want false")
		}
	})

	t.Run("invalid format returns error", func(t *testing.T) {
		t.Setenv(envKey, "abc")
		_, set, err := optionalEnvDuration(envKey)
		if err == nil {
			t.Fatal("optionalEnvDuration() error = nil, want non-nil")
		}

		if !set {
			t.Fatal("optionalEnvDuration() set = false, want true")
		}
	})

	t.Run("non-positive duration returns error", func(t *testing.T) {
		t.Setenv(envKey, "0s")
		_, set, err := optionalEnvDuration(envKey)
		if err == nil {
			t.Fatal("optionalEnvDuration() error = nil, want non-nil")
		}

		if !set {
			t.Fatal("optionalEnvDuration() set = false, want true")
		}
	})

	t.Run("valid duration", func(t *testing.T) {
		t.Setenv(envKey, "45s")
		got, set, err := optionalEnvDuration(envKey)
		if err != nil {
			t.Fatalf("optionalEnvDuration() error = %v, want nil", err)
		}

		if !set {
			t.Fatal("optionalEnvDuration() set = false, want true")
		}

		if got != 45*time.Second {
			t.Fatalf("optionalEnvDuration() duration = %v, want %v", got, 45*time.Second)
		}
	})
}
