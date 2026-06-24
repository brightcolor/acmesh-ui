package acme

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Client runs acme.sh commands. It never invokes a shell; only the configured
// binary is executed with an explicit argv.
type Client struct {
	Binary  string
	Home    string
	Builder Builder
}

// NewClient constructs a Client.
func NewClient(binary, home string, b Builder) *Client {
	return &Client{Binary: binary, Home: home, Builder: b}
}

// Result is the outcome of a command run.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// CheckResult reports the health of the acme.sh installation.
type CheckResult struct {
	BinaryPath   string `json:"binary_path"`
	BinaryExists bool   `json:"binary_exists"`
	Executable   bool   `json:"executable"`
	Version      string `json:"version,omitempty"`
	Home         string `json:"home"`
	HomeReadable bool   `json:"home_readable"`
	Error        string `json:"error,omitempty"`
}

// Check verifies the binary and home directory without running an issuance.
func (c *Client) Check(ctx context.Context) CheckResult {
	res := CheckResult{BinaryPath: c.Binary, Home: c.Home}

	info, err := os.Stat(c.Binary)
	if err != nil {
		res.Error = fmt.Sprintf("acme.sh binary not found at %s", c.Binary)
		return res
	}
	res.BinaryExists = true
	res.Executable = info.Mode()&0o111 != 0

	if hi, err := os.Stat(c.Home); err == nil && hi.IsDir() {
		if f, err := os.Open(c.Home); err == nil {
			f.Close()
			res.HomeReadable = true
		}
	}

	if res.Executable {
		if v, err := c.VersionString(ctx); err == nil {
			res.Version = v
		}
	}
	return res
}

// VersionString runs `acme.sh --version` and returns the parsed version line.
func (c *Client) VersionString(ctx context.Context) (string, error) {
	r, err := c.Run(ctx, c.Builder.Version(), nil, nil)
	if err != nil {
		return "", err
	}
	return ParseVersion(r.Stdout), nil
}

// Run executes cmd, streaming combined output line-by-line to onLine (if set)
// and returning the full captured Result. extraEnv is appended to the process
// environment (used for DNS provider variables).
func (c *Client) Run(ctx context.Context, cmd Command, extraEnv []string, onLine func(string)) (Result, error) {
	if c.Binary == "" {
		return Result{}, fmt.Errorf("acme.sh binary path is not configured")
	}

	start := time.Now()
	ec := exec.CommandContext(ctx, c.Binary, cmd.Args...)
	ec.Env = append(os.Environ(), append(cmd.Env, extraEnv...)...)
	if c.Home != "" {
		ec.Env = append(ec.Env, "LE_WORKING_DIR="+c.Home)
	}
	ec.Dir = c.Home

	var stdout, stderr bytes.Buffer
	if onLine != nil {
		ec.Stdout = io.MultiWriter(&stdout, lineWriter(onLine))
		ec.Stderr = io.MultiWriter(&stderr, lineWriter(onLine))
	} else {
		ec.Stdout = &stdout
		ec.Stderr = &stderr
	}

	err := ec.Run()
	res := Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
		ExitCode: 0,
	}
	if err != nil {
		var ee *exec.ExitError
		if ok := asExitError(err, &ee); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
			return res, err
		}
	}
	return res, nil
}

// lineWriter splits writes on newlines and invokes fn per line.
type lineWriterFunc struct {
	fn  func(string)
	buf []byte
}

func lineWriter(fn func(string)) io.Writer { return &lineWriterFunc{fn: fn} }

func (w *lineWriterFunc) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(string(w.buf[:i]), "\r")
		w.fn(line)
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}
