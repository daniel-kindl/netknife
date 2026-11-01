// Package ping runs Linux iputils ping and returns structured probe statistics.
package ping

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	MAX_COUNT        = 2147483647
	MAX_OUTPUT_BYTES = 64 * 1024
	PIPE_WAIT_DELAY  = 100 * time.Millisecond
)

// Result describes a completed set of ICMP echo probes.
type Result struct {
	Host        string  `json:"host"`
	Transmitted int     `json:"transmitted"`
	Received    int     `json:"received"`
	LossPercent float64 `json:"loss_percent"`
	RTT         RTT     `json:"rtt"`
}

// RTT contains round-trip statistics in milliseconds.
type RTT struct {
	MinMS  float64 `json:"min_ms"`
	AvgMS  float64 `json:"avg_ms"`
	MaxMS  float64 `json:"max_ms"`
	MdevMS float64 `json:"mdev_ms"`
}

// Validate accepts one IP address or ASCII DNS host and an iputils packet count.
func Validate(host string, count int) error {
	if count < 1 || int64(count) > MAX_COUNT {
		return errors.New("count must be between 1 and 2147483647")
	}
	if address, err := netip.ParseAddr(host); err == nil {
		for _, ch := range address.Zone() {
			if !asciiLetterOrDigit(ch) && ch != '_' && ch != '-' && ch != '.' {
				return errors.New("invalid IP address zone")
			}
		}
		return nil
	}
	name := strings.TrimSuffix(host, ".")
	if len(name) == 0 || len(name) > 253 {
		return errors.New("host must be an IP address or ASCII DNS name")
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("host must be an IP address or ASCII DNS name")
		}
		for _, ch := range label {
			if !asciiLetterOrDigit(ch) && ch != '-' {
				return errors.New("host must be an IP address or ASCII DNS name")
			}
		}
	}
	return nil
}

func asciiLetterOrDigit(ch rune) bool {
	return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9'
}

// Run requires validated arguments. It never invokes a shell or elevates privileges.
// The context limits the entire subprocess, including name resolution.
func Run(ctx context.Context, host string, count int) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	cmd := exec.CommandContext(ctx, "ping", "-n", "-q", "-c", strconv.Itoa(count), "-W", "1", "--", host)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.WaitDelay = PIPE_WAIT_DELAY
	var stdout, stderr limitedOutput
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{}, ctxErr
	}
	if errors.Is(err, exec.ErrNotFound) {
		return Result{}, errors.New("ping tool not found. Install iputils or check ping permissions and PATH")
	}
	if errors.Is(err, os.ErrPermission) {
		return Result{}, fmt.Errorf("execute ping: check executable permissions: %w", err)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			result, parseErr := parseSummary(stdout.String(), host, count)
			if parseErr == nil && result.Received == 0 {
				return Result{}, fmt.Errorf("no ICMP replies from %q. The target may be unreachable or ICMP may be blocked", host)
			}
		}
		detail := strings.TrimSpace(stderr.String())
		if strings.Contains(strings.ToLower(detail), "permission") || strings.Contains(strings.ToLower(detail), "operation not permitted") {
			return Result{}, fmt.Errorf("ping failed. Check ICMP permissions: %q", detail)
		}
		if detail != "" {
			return Result{}, fmt.Errorf("ping %q failed: %q: %w", host, detail, err)
		}
		return Result{}, fmt.Errorf("ping %q failed: %w", host, err)
	}
	result, err := parseSummary(stdout.String(), host, count)
	if err != nil {
		return Result{}, err
	}
	if result.Received == 0 {
		return Result{}, fmt.Errorf("no ICMP replies from %q. The target may be unreachable or ICMP may be blocked", host)
	}
	return result, nil
}

// limitedOutput drains tool output while retaining at most 64 KiB per stream.
type limitedOutput struct{ buffer bytes.Buffer }

func (out *limitedOutput) String() string { return out.buffer.String() }

func (out *limitedOutput) Write(p []byte) (int, error) {
	size := len(p)
	remaining := MAX_OUTPUT_BYTES - out.buffer.Len()
	if len(p) > remaining {
		p = p[:remaining]
	}
	out.buffer.Write(p)
	return size, nil
}

var packetSummary = regexp.MustCompile(`(?m)^([0-9]+) packets transmitted, ([0-9]+) received,`)
var rttSummary = regexp.MustCompile(`(?m)^rtt min/avg/max/mdev = ([0-9.]+)/([0-9.]+)/([0-9.]+)/([0-9.]+) ms(?:, pipe [0-9]+)?$`)

func parseSummary(output, host string, count int) (Result, error) {
	matches := packetSummary.FindStringSubmatch(output)
	if matches == nil {
		return Result{}, errors.New("invalid ping summary. Linux iputils ping is required")
	}
	sent, err := strconv.Atoi(matches[1])
	if err != nil {
		return Result{}, errors.New("invalid packet counts in ping output")
	}
	received, err := strconv.Atoi(matches[2])
	if err != nil || sent != count || received > sent {
		return Result{}, errors.New("invalid packet counts in ping output")
	}
	result := Result{Host: host, Transmitted: sent, Received: received, LossPercent: 100 * float64(sent-received) / float64(sent)}
	if received == 0 {
		return result, nil
	}
	matches = rttSummary.FindStringSubmatch(output)
	if matches == nil {
		return Result{}, errors.New("invalid ping RTT summary. Linux iputils ping is required")
	}
	values := make([]float64, 4)
	for i := range values {
		values[i], err = strconv.ParseFloat(matches[i+1], 64)
		if err != nil {
			return Result{}, errors.New("invalid ping RTT summary")
		}
	}
	if values[0] > values[1] || values[1] > values[2] {
		return Result{}, errors.New("invalid ping RTT summary: minimum must not exceed average, and average must not exceed maximum")
	}
	result.RTT = RTT{MinMS: values[0], AvgMS: values[1], MaxMS: values[2], MdevMS: values[3]}
	return result, nil
}
