package remote

import "testing"

func TestShellQuoteArgs(t *testing.T) {
	got := shellQuoteArgs([]string{"--execution-id", "abc 123", "has'quote"})
	want := "'--execution-id' 'abc 123' 'has'\"'\"'quote'"
	if got != want {
		t.Fatalf("shellQuoteArgs() = %q, want %q", got, want)
	}
}
