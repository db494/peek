package ec2

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type Instance struct {
	ID        string
	Name      string
	Type      string
	PrivateIP string
	Platform  string
	AMIID     string
	State     string
}

func List(ctx context.Context, cfg aws.Config) ([]Instance, error) {
	client := ec2.NewFromConfig(cfg)

	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"pending", "running", "shutting-down", "stopping", "stopped"},
			},
		},
	}

	var instances []Instance
	paginator := ec2.NewDescribeInstancesPaginator(client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, reservation := range page.Reservations {
			for _, inst := range reservation.Instances {
				instances = append(instances, toInstance(inst))
			}
		}
	}
	return instances, nil
}

func toInstance(inst types.Instance) Instance {
	i := Instance{
		ID:    aws.ToString(inst.InstanceId),
		Type:  string(inst.InstanceType),
		AMIID: aws.ToString(inst.ImageId),
	}
	if inst.PrivateIpAddress != nil {
		i.PrivateIP = *inst.PrivateIpAddress
	}
	if inst.PlatformDetails != nil {
		i.Platform = *inst.PlatformDetails
	}
	if inst.State != nil {
		i.State = string(inst.State.Name)
	}
	for _, tag := range inst.Tags {
		if aws.ToString(tag.Key) == "Name" {
			i.Name = aws.ToString(tag.Value)
			break
		}
	}
	return i
}
