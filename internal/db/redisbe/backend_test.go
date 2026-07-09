package redisbe

import (
	"strings"
	"testing"
)

func TestConnectRejectsInvalidDbNumber(t *testing.T) {
	tests := []struct {
		name   string
		rdbNum string
	}{
		{"not a number", "abc"},
		{"empty", ""},
		{"float", "1.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Connect(Config{Host: "127.0.0.1", Port: "6379", RdbNum: tt.rdbNum})
			if err == nil {
				t.Fatal("expected error for invalid db number")
			}
			if !strings.Contains(err.Error(), "invalid redis db number") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFormatJson(t *testing.T) {
	got, err := FormatJson(map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("FormatJson: %v", err)
	}
	want := "{\n  \"k\": \"v\"\n}"
	if got != want {
		t.Fatalf("FormatJson = %q, want %q", got, want)
	}
}
