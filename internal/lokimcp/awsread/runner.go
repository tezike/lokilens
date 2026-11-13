package awsread

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"time"
)

const (
	maxStdoutBytes = 20 << 20
	maxStderrRead  = 64 << 10
)

type Runner interface {
	Run(ctx context.Context, name string, args []string) (stdout, stderr []byte, err error)
}

type ExecRunner struct {
	Timeout time.Duration
}

type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	room := c.limit - c.buf.Len()
	if room <= 0 {
		c.truncated = true
		return len(p), nil
	}
	if len(p) > room {
		c.buf.Write(p[:room])
		c.truncated = true
		return len(p), nil
	}
	c.buf.Write(p)
	return len(p), nil
}

func (r ExecRunner) Run(ctx context.Context, name string, args []string) ([]byte, []byte, error) {
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "AWS_PAGER=")
	stdout := &cappedBuffer{limit: maxStdoutBytes}
	stderr := &cappedBuffer{limit: maxStderrRead}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil && stdout.truncated {
		err = NewAWSError("output exceeded 20MB limit; narrow with filters or MaxResults", nil, nil)
	}
	if ctx.Err() == context.DeadlineExceeded {
		err = NewAWSError("command timed out", stderr.buf.Bytes(), ctx.Err())
	}
	return stdout.buf.Bytes(), stderr.buf.Bytes(), err
}
