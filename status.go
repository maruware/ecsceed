package ecsceed

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/fatih/color"
	"github.com/pkg/errors"
)

var spcIndent = "\t"

func formatTaskSet(d *ecs.TaskSet) string {
	return fmt.Sprintf(
		"%8s %s desired:%d pending:%d running:%d",
		*d.Status,
		arnToName(*d.TaskDefinition),
		*d.ComputedDesiredCount, *d.PendingCount, *d.RunningCount,
	)
}

func formatTask(t *ecs.Task, ci *ecs.ContainerInstance) string {
	common := fmt.Sprintf(
		"%8s %s %s",
		*t.LastStatus,
		arnToName(*t.TaskArn),
		arnToName(*t.TaskDefinitionArn),
	)
	if *t.LaunchType == "EC2" {
		return fmt.Sprintf("%s type:%s(%s)", common, *t.LaunchType, *ci.Ec2InstanceId)
	}
	return fmt.Sprintf(
		"%s type:%s",
		common,
		*t.LaunchType,
	)
}

func formatScalingPolicy(p *applicationautoscaling.ScalingPolicy) string {
	return fmt.Sprintf("  Policy name:%s type:%s", *p.PolicyName, *p.PolicyType)
}

func formatScalableTarget(t *applicationautoscaling.ScalableTarget) string {
	return strings.Join([]string{
		fmt.Sprintf(
			spcIndent+"Capacity min:%d max:%d",
			*t.MinCapacity,
			*t.MaxCapacity,
		),
		fmt.Sprintf(
			spcIndent+"Suspended in:%t out:%t scheduled:%t",
			*t.SuspendedState.DynamicScalingInSuspended,
			*t.SuspendedState.DynamicScalingOutSuspended,
			*t.SuspendedState.ScheduledScalingSuspended,
		),
	}, "\n")
}

func (a *App) describeAutoScaling(s *ecs.Service) error {
	resouceID := fmt.Sprintf("service/%s/%s", arnToName(*s.ClusterArn), *s.ServiceName)
	tout, err := a.autoScaling.DescribeScalableTargets(
		&applicationautoscaling.DescribeScalableTargetsInput{
			ResourceIds:       []*string{&resouceID},
			ServiceNamespace:  aws.String("ecs"),
			ScalableDimension: aws.String("ecs:service:DesiredCount"),
		},
	)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "AccessDeniedException" {
				a.DebugLog("unable to describe scalable targets. requires IAM for application-autoscaling:Describe* to display informations about auto-scaling.")
				return nil
			}
		}
		return errors.Wrap(err, "failed to describe scalable targets")
	}
	if len(tout.ScalableTargets) == 0 {
		return nil
	}

	fmt.Println("AutoScaling:")
	for _, target := range tout.ScalableTargets {
		fmt.Println(formatScalableTarget(target))
	}

	pout, err := a.autoScaling.DescribeScalingPolicies(
		&applicationautoscaling.DescribeScalingPoliciesInput{
			ResourceId:        &resouceID,
			ServiceNamespace:  aws.String("ecs"),
			ScalableDimension: aws.String("ecs:service:DesiredCount"),
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to describe scaling policies")
	}
	for _, policy := range pout.ScalingPolicies {
		fmt.Println(formatScalingPolicy(policy))
	}
	return nil
}

func (a *App) servicesStatus(ctx context.Context, names []*string, events int) ([]*ecs.Service, error) {
	desc, err := a.DescribeServices(ctx, names)

	if err != nil {
		return nil, err
	}

	for i, s := range desc.Services {
		fmt.Println("Service:", LogTarget(*s.ServiceName))
		fmt.Println("TaskDefinition:", LogTarget(arnToName(*s.TaskDefinition)))
		if len(s.Deployments) > 0 {
			fmt.Println("Deployments:")
			for _, dep := range s.Deployments {
				fmt.Println(spcIndent + formatDeployment(dep))
			}
		}
		if len(s.TaskSets) > 0 {
			fmt.Println("TaskSets:")
			for _, ts := range s.TaskSets {
				fmt.Println(spcIndent + formatTaskSet(ts))
			}
		}

		tasks, err := a.ListServiceTasks(ctx, *s.ServiceName)
		if err != nil {
			return nil, err
		}

		fmt.Println("Tasks:")
		for _, task := range tasks {
			var instance *ecs.ContainerInstance
			if task.ContainerInstanceArn != nil {
				instance, err = a.DescribeContainerInstance(ctx, *task.ContainerInstanceArn)
				if err != nil {
					return nil, err
				}
			}
			fmt.Println(spcIndent + formatTask(task, instance))
		}

		if err := a.describeAutoScaling(s); err != nil {
			return nil, errors.Wrap(err, "failed to describe autoscaling")
		}

		fmt.Println("Events:")
		for i, event := range s.Events {
			if i >= events {
				break
			}
			for _, line := range formatEvent(event, TerminalWidth) {
				fmt.Println(spcIndent + line)
			}
		}

		if i != len(desc.Services)-1 {
			fmt.Println()
		}
	}

	return desc.Services, nil
}

func (a *App) clusterStatus(ctx context.Context) error {
	cluster, err := a.DescribeCluster(ctx, a.def.cluster)
	if err != nil {
		return err
	}
	fmt.Printf("Tasks count: %d\n", *cluster.RunningTasksCount)
	fmt.Printf("Status: %s\n", *cluster.Status)

	return nil
}

type StatusOption struct {
	Events int
}

func (a *App) Status(ctx context.Context, opt StatusOption) error {
	err := a.ResolveConfigStack(Params{})
	if err != nil {
		return err
	}

	srvNames := []*string{}
	for name := range a.def.nameToSrv {
		srvNames = append(srvNames, aws.String(a.resolveFullName(name)))
	}

	printSection := color.New(color.FgGreen, color.Bold)
	printSection.Println(">> Services")

	_, err = a.servicesStatus(ctx, srvNames, opt.Events)
	if err != nil {
		return err
	}

	fmt.Println()

	printSection.Println(">> Cluster")
	err = a.clusterStatus(ctx)
	if err != nil {
		return err
	}

	return nil

}
