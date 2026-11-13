package awsread

import (
	"fmt"
	"strings"
)

const maxStderrBytes = 2048

type AWSError struct {
	Message string
	Stderr  string
	Err     error
}

func (e *AWSError) Error() string {
	msg := "aws cli error: " + e.Message
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *AWSError) Unwrap() error { return e.Err }

func NewAWSError(msg string, stderr []byte, err error) *AWSError {
	return &AWSError{Message: msg, Stderr: trimStderr(stderr), Err: err}
}

func trimStderr(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > maxStderrBytes {
		s = strings.ToValidUTF8(s[:maxStderrBytes], "") + "...(truncated)"
	}
	return s
}

func invalid(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
