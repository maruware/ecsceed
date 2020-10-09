package ecsceed

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
)

type LogsOption struct {
	AdditionalParams Params
	ContainerName    string
}

func (a *App) Logs(ctx context.Context, name string, opt LogsOption) error {
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

	tdArn := srv.TaskDefinition
	td, err := a.DescribeTaskDefinition(ctx, *tdArn)
	if err != nil {
		return err
	}
	container := containerOf(td, &opt.ContainerName)

	a.Log("container", LogTarget(*container.Name))

	tasks, err := a.ListServiceTasks(ctx, *srv.ServiceName)
	if err != nil {
		return err
	}

	now := time.Now()

	for _, task := range tasks {
		go a.WatchLogs(ctx, task, container, now)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	}
}
