package main

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestRun(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		wantOut  string
		wantErr  string
		wantCode int
	}{
		{"help", nil, help, "", 0},
		{"help flag", []string{"--help"}, help, "", 0},
		{"version", []string{"version"}, "dev\n", "", 0},
		{"version JSON", []string{"version", "--json"}, "{\"command\":\"version\",\"data\":\"dev\"}\n", "", 0},
		{"unknown", []string{"unknown"}, "", "netknife: invalid arguments: [\"unknown\"]\nRun 'netknife --help' for usage.\n", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run(context.Background(), tc.args, &out, &errOut)
			if code != tc.wantCode || out.String() != tc.wantOut || errOut.String() != tc.wantErr {
				t.Errorf("got code %d, stdout %q, stderr %q", code, out.String(), errOut.String())
			}
		})
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errOut bytes.Buffer
	if code := run(ctx, nil, &out, &errOut); code != 1 || out.String() != "" || errOut.String() != "netknife: cancelled\n" {
		t.Fatalf("got code %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
}

func TestRenderJSON(t *testing.T) {
	var out bytes.Buffer
	if err := render(&out, result{Command: "example", Data: "value"}, true); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "{\"command\":\"example\",\"data\":\"value\"}\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestOutputFailure(t *testing.T) {
	var errOut bytes.Buffer
	if code := run(context.Background(), nil, failingWriter{}, &errOut); code != 1 || errOut.String() != "netknife: write failed\n" {
		t.Fatalf("got code %d, stderr %q", code, errOut.String())
	}
}
