package awsread

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/olusolaa/lokilens/internal/lokimcp/audit"
)

var (
	operationPattern = regexp.MustCompile(`^(describe|list|get|batch-get)-[a-z0-9-]+$`)
	regionPattern    = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d$`)

	allowedServices = []string{"ec2", "elbv2", "autoscaling", "deploy", "route53"}

	deniedOperations = map[string]map[string]bool{
		"ec2": {
			"describe-instance-attribute":       true,
			"describe-launch-template-versions": true,
			"get-launch-template-data":          true,
			"get-console-output":                true,
			"get-console-screenshot":            true,
			"get-password-data":                 true,
		},
	}
)

func Services() []string { return append([]string(nil), allowedServices...) }

type ToolHandlers struct {
	runner  Runner
	cliPath string
	region  string
	audit   *audit.Logger
}

func NewToolHandlers(runner Runner, cliPath, region string, auditLogger *audit.Logger) *ToolHandlers {
	return &ToolHandlers{runner: runner, cliPath: cliPath, region: region, audit: auditLogger}
}

func (h *ToolHandlers) fail(tool string, start time.Time, err error) error {
	h.audit.ToolFailed(tool, time.Since(start).Milliseconds(), err)
	return err
}

func (h *ToolHandlers) AWSRead(ctx context.Context, in ReadInput) (ReadOutput, error) {
	start := time.Now()
	out := ReadOutput{Service: in.Service, Operation: in.Operation}

	if !serviceAllowed(in.Service) {
		return out, h.fail("aws_read", start, invalid("service %q is not supported; use one of: %s", in.Service, strings.Join(allowedServices, ", ")))
	}
	if !operationPattern.MatchString(in.Operation) {
		return out, h.fail("aws_read", start, invalid("operation %q must be a read-only AWS CLI operation matching %s", in.Operation, operationPattern.String()))
	}
	if deniedOperations[in.Service][in.Operation] {
		return out, h.fail("aws_read", start, invalid("operation %s %s is blocked because it can expose user data or secrets", in.Service, in.Operation))
	}
	region := strings.TrimSpace(in.Region)
	if region == "" {
		region = h.region
	}
	if !regionPattern.MatchString(region) {
		return out, h.fail("aws_read", start, invalid("region %q is not a valid AWS region, e.g. eu-west-1", region))
	}
	out.Region = region

	paramsJSON, err := encodeParams(in.Params)
	if err != nil {
		return out, h.fail("aws_read", start, err)
	}

	args := []string{in.Service, in.Operation}
	if paramsJSON != "" {
		args = append(args, "--cli-input-json", paramsJSON)
	}
	args = append(args, "--output", "json", "--no-cli-pager", "--region", region)

	stdout, stderr, err := h.runner.Run(ctx, h.cliPath, args)
	if err != nil {
		return out, h.fail("aws_read", start, NewAWSError(fmt.Sprintf("%s %s failed", in.Service, in.Operation), stderr, err))
	}

	if len(bytes.TrimSpace(stdout)) > 0 {
		if err := json.Unmarshal(stdout, &out.Result); err != nil {
			return out, h.fail("aws_read", start, fmt.Errorf("aws cli returned invalid JSON: %w", err))
		}
	}

	h.audit.ToolInvoked("aws_read", time.Since(start).Milliseconds(),
		slog.String("service", in.Service), slog.String("operation", in.Operation), slog.String("region", region))
	return out, nil
}

func serviceAllowed(s string) bool {
	for _, a := range allowedServices {
		if a == s {
			return true
		}
	}
	return false
}

func encodeParams(p any) (string, error) {
	switch v := p.(type) {
	case nil:
		return "", nil
	case map[string]any:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("params could not be encoded: %w", err)
		}
		return string(b), nil
	case string:
		if strings.TrimSpace(v) == "" {
			return "", nil
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(v), &m); err != nil || m == nil {
			return "", fmt.Errorf("params must be a JSON object")
		}
		b, err := json.Marshal(m)
		if err != nil {
			return "", fmt.Errorf("params could not be encoded: %w", err)
		}
		return string(b), nil
	default:
		return "", fmt.Errorf("params must be a JSON object")
	}
}
