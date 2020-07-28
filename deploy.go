package ecsceed

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

type DeployOption struct {
	UpdateService      bool
	ForceNewDeployment *bool
	AdditionalParams   Params
}

func tdToRegisterTaskDefinitionInput(td *ecs.TaskDefinition) *ecs.RegisterTaskDefinitionInput {
	return &ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions:    td.ContainerDefinitions,
		Cpu:                     td.Cpu,
		ExecutionRoleArn:        td.ExecutionRoleArn,
		Family:                  td.Family,
		Memory:                  td.Memory,
		NetworkMode:             td.NetworkMode,
		PlacementConstraints:    td.PlacementConstraints,
		RequiresCompatibilities: td.RequiresCompatibilities,
		TaskRoleArn:             td.TaskRoleArn,
		ProxyConfiguration:      td.ProxyConfiguration,
		Volumes:                 td.Volumes,
	}
}

func srvToUpdateServiceInput(srv *ecs.Service) *ecs.UpdateServiceInput {
	return &ecs.UpdateServiceInput{
		CapacityProviderStrategy:      srv.CapacityProviderStrategy,
		DeploymentConfiguration:       srv.DeploymentConfiguration,
		HealthCheckGracePeriodSeconds: srv.HealthCheckGracePeriodSeconds,
		NetworkConfiguration:          srv.NetworkConfiguration,
		PlacementConstraints:          srv.PlacementConstraints,
		PlacementStrategy:             srv.PlacementStrategy,
		PlatformVersion:               srv.PlatformVersion,
	}
}

func (a *App) RegisterTaskDefinition(ctx context.Context, td *ecs.TaskDefinition) (*ecs.TaskDefinition, error) {
	out, err := a.ecs.RegisterTaskDefinitionWithContext(
		ctx,
		tdToRegisterTaskDefinitionInput(td),
	)
	if err != nil {
		return nil, err
	}
	return out.TaskDefinition, nil
}

func (a *App) UpdateServiceAttributes(ctx context.Context, srv *ecs.Service, name string, cluster string, forceNewDeployment *bool) (*ecs.Service, error) {
	in := srvToUpdateServiceInput(srv)
	in.ForceNewDeployment = forceNewDeployment

	in.Service = aws.String(name)
	in.Cluster = aws.String(cluster)

	out, err := a.ecs.UpdateServiceWithContext(ctx, in)
	if err != nil {
		return nil, err
	}
	// time.Sleep(delayForServiceChanged) // wait for service updated
	sv := out.Service

	return sv, nil
}

func (a *App) UpdateServiceTask(ctx context.Context, name string, cluster string, taskDefinitionArn string, count *int64, forceNewDeployment *bool) error {
	in := &ecs.UpdateServiceInput{
		Service:            aws.String(name),
		Cluster:            aws.String(cluster),
		TaskDefinition:     aws.String(taskDefinitionArn),
		DesiredCount:       count,
		ForceNewDeployment: forceNewDeployment,
	}
	// msg := "Updating service tasks"
	// if *opt.ForceNewDeployment {
	// 	msg = msg + " with force new deployment"
	// }
	// msg = msg + "..."
	// d.Log(msg)
	// d.DebugLog(in.String())

	_, err := a.ecs.UpdateServiceWithContext(ctx, in)
	if err != nil {
		return err
	}
	// time.Sleep(delayForServiceChanged) // wait for service updated
	return nil
}

func (a *App) Deploy(ctx context.Context, opt DeployOption) error {
	err := a.ResolveConfigStack(opt.AdditionalParams)
	if err != nil {
		return err
	}

	nameToTdArn := map[string]string{}
	// register task def
	for name, td := range a.nameToTd {
		newTd, err := a.RegisterTaskDefinition(ctx, &td)
		if err != nil {
			return err
		}
		tdArn := *newTd.TaskDefinitionArn
		nameToTdArn[name] = tdArn
	}

	// update service
	for name, srv := range a.nameToSrv {
		tdArn, ok := nameToTdArn[srv.taskDefinition]
		if !ok {
			return fmt.Errorf("Bad reference service to task definition")
		}
		err := a.UpdateServiceTask(ctx, name, a.cluster, tdArn, nil, opt.ForceNewDeployment)
		if err != nil {
			return err
		}
		if opt.UpdateService {
			_, err := a.UpdateServiceAttributes(ctx, &srv.srv, name, a.cluster, opt.ForceNewDeployment)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
