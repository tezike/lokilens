package awsread

import (
	"context"
	"strings"
	"testing"
)

func TestExecRunner_EnvAndOutput(t *testing.T) {
	out, _, err := ExecRunner{}.Run(context.Background(), "sh", []string{"-c", `printf "[%s]" "${AWS_PAGER-unset}"`})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "[]" {
		t.Fatalf("out = %q", out)
	}
}

func TestExecRunner_NonZeroExitReturnsStderr(t *testing.T) {
	_, stderr, err := ExecRunner{}.Run(context.Background(), "sh", []string{"-c", "echo boom >&2; exit 3"})
	if err == nil || !strings.Contains(string(stderr), "boom") {
		t.Fatalf("err = %v stderr = %q", err, stderr)
	}
}
