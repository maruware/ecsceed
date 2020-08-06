package ecsceed

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

func (a *App) DescribeLogGroups(ctx context.Context, prefix string) ([]*cloudwatchlogs.LogGroup, error) {
	out, err := a.cwl.DescribeLogGroupsWithContext(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		Limit:              aws.Int64(1),
		LogGroupNamePrefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}

	return out.LogGroups, nil
}

func (a *App) CreateLogGroup(ctx context.Context, name string) error {
	_, err := a.cwl.CreateLogGroupWithContext(ctx, &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(name),
	})

	return err
}
