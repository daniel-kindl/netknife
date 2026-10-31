package main

import (
	"bytes"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var out bytes.Buffer
	if err := run(&out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "netknife: network diagnostics toolkit\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
