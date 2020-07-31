package ecsceed

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

type RunOption struct {
	NoWait             bool
	AdditionalParams   Params
	Count              int64
	TaskDefinitionPath string
	Command            []string
	Overrides          string
	ContainerName      string
}

func containerOf(td *ecs.TaskDefinition, name *string) *ecs.ContainerDefinition {
	if name == nil || *name == "" {
		return td.ContainerDefinitions[0]
	}
	for _, c := range td.ContainerDefinitions {
		if *c.Name == *name {
			c := c
			return c
		}
	}
	return nil
}

func (a *App) Run(ctx context.Context, name string, opt RunOption) error {
	a.Log("base service", LogTarget(name))
	err := a.ResolveConfigStack(opt.AdditionalParams)
	if err != nil {
		return err
	}

	if _, ok := a.def.nameToSrv[name]; !ok {
		return fmt.Errorf("service %s is undefined", name)
	}

	descSrvs, err := a.DescribeServices(ctx, []*string{aws.String(name)})
	if err != nil {
		return err
	}

	srv := descSrvs.Services[0]

	var tdArn *string
	var container *ecs.ContainerDefinition

	if len(opt.TaskDefinitionPath) > 0 {
		// extend
		var td ecs.TaskDefinition
		path, err := filepath.Abs(opt.TaskDefinitionPath)
		if err != nil {
			return err
		}
		err = loadAndMatchTmpl(path, a.def.params, &td)
		if err != nil {
			return err
		}
		newTd, err := a.RegisterTaskDefinition(ctx, &td)
		if err != nil {
			return err
		}
		tdArn = newTd.TaskDefinitionArn
		container = containerOf(newTd, &opt.ContainerName)
	} else {
		tdArn = srv.TaskDefinition
		td, err := a.DescribeTaskDefinition(ctx, *tdArn)
		if err != nil {
			return err
		}
		container = containerOf(td, &opt.ContainerName)
	}

	a.Log("container", LogTarget(*container.Name))

	count := opt.Count

	var ov ecs.TaskOverride
	if ovStr := opt.Overrides; ovStr != "" {
		if err := json.Unmarshal([]byte(ovStr), &ov); err != nil {
			return fmt.Errorf("invalid overrides: %w", err)
		}
	}
	if opt.Command != nil {
		a.Log("command", LogTarget(opt.Command))
		cmd := aws.StringSlice(opt.Command)
		ov.ContainerOverrides = []*ecs.ContainerOverride{
			{
				Name:    container.Name,
				Command: cmd,
			},
		}
	}

	task, err := a.RunTask(ctx, *srv, *tdArn, count, &ov)
	if err != nil {
		return err
	}

	if !opt.NoWait {
		if err := a.WaitRunTask(ctx, task, container, time.Now()); err != nil {
			return fmt.Errorf("failed to run task: %w", err)
		}
	}

	if err := a.DescribeTaskStatus(ctx, task, container); err != nil {
		return err
	}

	a.Log("Run task completed!")

	return nil
}
