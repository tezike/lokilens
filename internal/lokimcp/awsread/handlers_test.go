package awsread

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/olusolaa/lokilens/internal/lokimcp/audit"
)

type fakeRunner struct {
	stdout []byte
	stderr []byte
	err    error
	name   string
	args   []string
	calls  int
}

func (f *fakeRunner) Run(ctx context.Context, name string, args []string) ([]byte, []byte, error) {
	f.calls++
	f.name = name
	f.args = args
	return f.stdout, f.stderr, f.err
}

func newTestHandlers(fr *fakeRunner) *ToolHandlers {
	return NewToolHandlers(fr, "aws", "eu-west-1", audit.New(slog.New(slog.NewTextHandler(io.Discard, nil))))
}

func TestAWSRead_ArgvWithoutParams(t *testing.T) {
	fr := &fakeRunner{stdout: []byte(`{"Clusters":[]}`)}
	out, err := newTestHandlers(fr).AWSRead(context.Background(), ReadInput{Service: "ec2", Operation: "describe-instances"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ec2", "describe-instances", "--output", "json", "--no-cli-pager", "--region", "eu-west-1"}
	if fr.name != "aws" || !reflect.DeepEqual(fr.args, want) {
		t.Fatalf("argv = %s %v, want %v", fr.name, fr.args, want)
	}
	if out.Region != "eu-west-1" || out.Service != "ec2" || out.Operation != "describe-instances" {
		t.Fatalf("out = %+v", out)
	}
}

func TestAWSRead_ArgvWithParamsAndRegion(t *testing.T) {
	fr := &fakeRunner{stdout: []byte(`{}`)}
	_, err := newTestHandlers(fr).AWSRead(context.Background(), ReadInput{
		Service:   "autoscaling",
		Operation: "describe-scaling-activities",
		Params:    map[string]any{"AutoScalingGroupName": "asg-1", "MaxRecords": float64(20)},
		Region:    "eu-central-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"autoscaling", "describe-scaling-activities", "--cli-input-json", `{"AutoScalingGroupName":"asg-1","MaxRecords":20}`, "--output", "json", "--no-cli-pager", "--region", "eu-central-1"}
	if !reflect.DeepEqual(fr.args, want) {
		t.Fatalf("args = %v, want %v", fr.args, want)
	}
}

func TestAWSRead_ParamsAsJSONString(t *testing.T) {
	fr := &fakeRunner{stdout: []byte(`{}`)}
	_, err := newTestHandlers(fr).AWSRead(context.Background(), ReadInput{
		Service: "elbv2", Operation: "describe-target-health", Params: `{"TargetGroupArn":"arn:x"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fr.args[3] != `{"TargetGroupArn":"arn:x"}` {
		t.Fatalf("args = %v", fr.args)
	}
}

func TestAWSRead_Rejections(t *testing.T) {
	cases := []struct {
		name string
		in   ReadInput
		want string
	}{
		{"bad service", ReadInput{Service: "s3", Operation: "list-buckets"}, "not supported"},
		{"empty service", ReadInput{Operation: "describe-instances"}, "not supported"},
		{"write op", ReadInput{Service: "ec2", Operation: "terminate-instances"}, "operation"},
		{"uppercase op", ReadInput{Service: "ec2", Operation: "Describe-Instances"}, "operation"},
		{"snake op", ReadInput{Service: "ec2", Operation: "describe_instances"}, "operation"},
		{"flag injection", ReadInput{Service: "ec2", Operation: "describe-instances --profile x"}, "operation"},
		{"bare prefix", ReadInput{Service: "ec2", Operation: "describe-"}, "operation"},
		{"array params", ReadInput{Service: "ec2", Operation: "describe-instances", Params: []any{1}}, "JSON object"},
		{"number params", ReadInput{Service: "ec2", Operation: "describe-instances", Params: float64(3)}, "JSON object"},
		{"string array params", ReadInput{Service: "ec2", Operation: "describe-instances", Params: `[1]`}, "JSON object"},
		{"garbage string params", ReadInput{Service: "ec2", Operation: "describe-instances", Params: `nope`}, "JSON object"},
		{"null string params", ReadInput{Service: "ec2", Operation: "describe-instances", Params: `null`}, "JSON object"},
		{"bad region", ReadInput{Service: "ec2", Operation: "describe-instances", Region: "eu-west"}, "region"},
		{"region injection", ReadInput{Service: "ec2", Operation: "describe-instances", Region: "eu-west-1 --profile x"}, "region"},
	}
	for _, d := range []string{"describe-instance-attribute", "describe-launch-template-versions", "get-launch-template-data", "get-console-output", "get-console-screenshot", "get-password-data"} {
		cases = append(cases, struct {
			name string
			in   ReadInput
			want string
		}{"denied " + d, ReadInput{Service: "ec2", Operation: d}, "user data or secrets"})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fr := &fakeRunner{}
			_, err := newTestHandlers(fr).AWSRead(context.Background(), tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
			if fr.calls != 0 {
				t.Fatal("runner must not be called on rejection")
			}
		})
	}
}

func TestAWSRead_DeniedOnlyAppliesToEC2(t *testing.T) {
	fr := &fakeRunner{stdout: []byte(`{}`)}
	if _, err := newTestHandlers(fr).AWSRead(context.Background(), ReadInput{Service: "autoscaling", Operation: "describe-launch-configurations"}); err != nil {
		t.Fatal(err)
	}
}

func TestAWSRead_SurfacesStderr(t *testing.T) {
	fr := &fakeRunner{stderr: []byte("An error occurred (UnauthorizedOperation): not allowed\n"), err: errors.New("exit status 254")}
	_, err := newTestHandlers(fr).AWSRead(context.Background(), ReadInput{Service: "ec2", Operation: "describe-instances"})
	if err == nil || !strings.Contains(err.Error(), "UnauthorizedOperation") || !strings.Contains(err.Error(), "exit status 254") {
		t.Fatalf("err = %v", err)
	}
}

func TestAWSRead_StderrTrimmedTo2KB(t *testing.T) {
	fr := &fakeRunner{stderr: []byte(strings.Repeat("x", 10000)), err: errors.New("exit status 1")}
	_, err := newTestHandlers(fr).AWSRead(context.Background(), ReadInput{Service: "ec2", Operation: "describe-instances"})
	if err == nil || len(err.Error()) > 2300 {
		t.Fatalf("err length = %d", len(err.Error()))
	}
}

func TestAWSRead_ParsesStdout(t *testing.T) {
	fr := &fakeRunner{stdout: []byte(`{"Reservations":[{"Instances":[{"InstanceId":"i-1"}]}]}`)}
	out, err := newTestHandlers(fr).AWSRead(context.Background(), ReadInput{Service: "ec2", Operation: "describe-instances"})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.Result.(map[string]any)
	if !ok || m["Reservations"] == nil {
		t.Fatalf("result = %#v", out.Result)
	}
}

func TestAWSRead_EmptyStdoutIsNull(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("  \n")}
	out, err := newTestHandlers(fr).AWSRead(context.Background(), ReadInput{Service: "route53", Operation: "list-hosted-zones"})
	if err != nil || out.Result != nil {
		t.Fatalf("out = %+v err = %v", out, err)
	}
}

func TestAWSRead_InvalidStdoutJSON(t *testing.T) {
	fr := &fakeRunner{stdout: []byte("not json")}
	if _, err := newTestHandlers(fr).AWSRead(context.Background(), ReadInput{Service: "ec2", Operation: "describe-instances"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestHealthCheck(t *testing.T) {
	fr := &fakeRunner{stdout: []byte(`{}`)}
	src := NewSource(fr, "aws", "eu-west-1", audit.New(slog.New(slog.NewTextHandler(io.Discard, nil))))
	if err := src.HealthCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"sts", "get-caller-identity", "--output", "json"}
	if !reflect.DeepEqual(fr.args, want) {
		t.Fatalf("args = %v", fr.args)
	}
	fr.err, fr.stderr = errors.New("exit status 253"), []byte("Unable to locate credentials")
	err := src.HealthCheck(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Unable to locate credentials") {
		t.Fatalf("err = %v", err)
	}
}
