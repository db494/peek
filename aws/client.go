package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
)

// Instance holds the EC2 metadata the TUI displays.
type Instance struct {
	ID        string
	Name      string
	State     string
	PrivateIP string
	PublicIP  string
}

// Client wraps an EC2 client bound to a profile and region.
type Client struct {
	ec2     *ec2.Client
	Profile string
	Region  string
}

// NewClient loads shared config for the given profile and builds an EC2
// client. If region is empty, the profile's configured region is used.
func NewClient(ctx context.Context, profile, region string) (*Client, error) {
	opts := []func(*config.LoadOptions) error{}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("no region configured for profile %q: set one in ~/.aws/config or pass --region", profile)
	}
	return &Client{
		ec2:     ec2.NewFromConfig(cfg),
		Profile: profile,
		Region:  cfg.Region,
	}, nil
}

// WithRegion returns a copy of the client targeting a different region.
func (c *Client) WithRegion(region string) *Client {
	return &Client{
		ec2: ec2.NewFromConfig(aws.Config{
			Region:      region,
			Credentials: c.ec2.Options().Credentials,
		}),
		Profile: c.Profile,
		Region:  region,
	}
}

// Instances fetches all non-terminated EC2 instances in the client's region.
func (c *Client) Instances(ctx context.Context) ([]Instance, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{{
			Name:   aws.String("instance-state-name"),
			Values: []string{"pending", "running", "stopping", "stopped"},
		}},
	}

	var instances []Instance
	paginator := ec2.NewDescribeInstancesPaginator(c.ec2, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, classifyError(err)
		}
		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				instances = append(instances, toInstance(inst))
			}
		}
	}

	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Name != instances[j].Name {
			return instances[i].Name < instances[j].Name
		}
		return instances[i].ID < instances[j].ID
	})
	return instances, nil
}

// Regions returns the regions enabled for the account, sorted by name.
func (c *Client) Regions(ctx context.Context) ([]string, error) {
	out, err := c.ec2.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, classifyError(err)
	}
	regions := make([]string, 0, len(out.Regions))
	for _, r := range out.Regions {
		regions = append(regions, aws.ToString(r.RegionName))
	}
	sort.Strings(regions)
	return regions, nil
}

func toInstance(inst ec2types.Instance) Instance {
	i := Instance{
		ID:        aws.ToString(inst.InstanceId),
		State:     string(inst.State.Name),
		PrivateIP: aws.ToString(inst.PrivateIpAddress),
		PublicIP:  aws.ToString(inst.PublicIpAddress),
	}
	for _, tag := range inst.Tags {
		if aws.ToString(tag.Key) == "Name" {
			i.Name = aws.ToString(tag.Value)
			break
		}
	}
	return i
}

// classifyError rewrites credential problems into messages a human can act
// on; other errors pass through unchanged.
func classifyError(err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "ExpiredToken", "ExpiredTokenException", "RequestExpired":
			return fmt.Errorf("AWS credentials have expired — refresh them (e.g. aws sso login) and try again")
		case "InvalidClientTokenId", "UnrecognizedClientException", "AuthFailure":
			return fmt.Errorf("AWS credentials are invalid for this profile — check ~/.aws/credentials")
		case "UnauthorizedOperation", "AccessDenied", "AccessDeniedException":
			return fmt.Errorf("access denied: the profile lacks permission for this EC2 call (%s)", apiErr.ErrorCode())
		}
	}
	msg := err.Error()
	if strings.Contains(msg, "failed to refresh cached credentials") ||
		strings.Contains(msg, "token has expired") ||
		strings.Contains(msg, "failed to retrieve credentials") {
		return fmt.Errorf("AWS credentials could not be refreshed — log in again (e.g. aws sso login) and retry")
	}
	return err
}
