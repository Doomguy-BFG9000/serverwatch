package service

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateGroupName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{
			name:  "valid",
			input: "Production",
			want:  "Production",
		},
		{
			name:  "trims surrounding whitespace",
			input: "  Production  ",
			want:  "Production",
		},
		{
			name:    "empty",
			input:   "",
			wantErr: ErrGroupNameRequired,
		},
		{
			name:    "whitespace only",
			input:   "   \t\n",
			wantErr: ErrGroupNameRequired,
		},
		{
			name:    "too long",
			input:   strings.Repeat("a", MaxGroupNameLength+1),
			wantErr: ErrGroupNameTooLong,
		},
		{
			name:  "maximum length",
			input: strings.Repeat("a", MaxGroupNameLength),
			want:  strings.Repeat("a", MaxGroupNameLength),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ValidateGroupName(test.input)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("name = %q, want %q", got, test.want)
			}
		})
	}
}
