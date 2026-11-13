package awsread

import (
	"context"
	"fmt"

	"github.com/olusolaa/lokilens/internal/lokimcp/audit"
)

type Source struct {
	handlers *ToolHandlers
}

func NewSource(runner Runner, cliPath, region string, auditLogger *audit.Logger) *Source {
	return &Source{handlers: NewToolHandlers(runner, cliPath, region, auditLogger)}
}

func (s *Source) Name() string        { return "AWS" }
func (s *Source) Instruction() string { return systemInstruction }
func (s *Source) Description() string {
	return "Read-only AWS CLI access to EC2, ELBv2, Auto Scaling, CodeDeploy and Route 53"
}
func (s *Source) Handlers() *ToolHandlers { return s.handlers }

func (s *Source) HealthCheck(ctx context.Context) error {
	_, stderr, err := s.handlers.runner.Run(ctx, s.handlers.cliPath, []string{"sts", "get-caller-identity", "--output", "json"})
	if err != nil {
		return fmt.Errorf("aws cli unavailable: %w", NewAWSError("sts get-caller-identity failed", stderr, err))
	}
	return nil
}

const systemInstruction = `You are inspecting AWS infrastructure through a read-only AWS CLI tool, aws_read.

Use it to inspect EC2 instances, security groups and subnets; ALBs and target health; Auto Scaling groups and scaling activities; CodeDeploy deployments and instances; and Route 53 records.

Services: ec2, elbv2, autoscaling, deploy, route53. Operations must be describe-*, list-*, get-* or batch-get-* in AWS CLI kebab-case. Pass request fields as the params JSON object (CLI input JSON, PascalCase keys).

Examples:
1. service=ec2 operation=describe-instances params={"Filters":[{"Name":"tag:Name","Values":["lokilens-0"]}]}
2. service=elbv2 operation=describe-target-health params={"TargetGroupArn":"..."}
3. service=autoscaling operation=describe-scaling-activities params={"AutoScalingGroupName":"...","MaxRecords":20}
4. service=deploy operation=list-deployments, then service=deploy operation=batch-get-deployments with the returned IDs

Use filters and MaxResults/MaxRecords to keep output small; large results are truncated.
IAM is read-only. S3 objects, secrets, SSM parameters and user data are blocked.`
