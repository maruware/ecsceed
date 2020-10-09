package ecsceed

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
)

type LogsOption struct {
	AdditionalParams Params
	ContainerName    string
	StartTime        string
	Tail             bool
}

var units = []string{"m", "h", "d"}

func parseStartTime(startTime string) (time.Time, error) {
	var unit string
	for _, u := range units {
		if strings.HasSuffix(startTime, u) {
			unit = u
			break
		}
	}

	if unit != "" {
		now := time.Now()
		nums := strings.TrimSuffix(startTime, unit)
		num, err := strconv.Atoi(nums)
		if err != nil {
			return time.Time{}, err
		}

		switch unit {
		case "m":
			return now.Add(time.Minute * time.Duration(-num)), nil
		case "h":
			return now.Add(time.Hour * time.Duration(-num)), nil
		case "d":
			return now.Add(time.Hour * time.Duration(-num*24)), nil
		}
	}

	return time.Time{}, fmt.Errorf("not implement a startTime format yet")
}

func (a *App) Logs(ctx context.Context, name string, opt LogsOption) error {
	var startTime time.Time
	if opt.StartTime != "" {
		t, err := parseStartTime(opt.StartTime)
		if err != nil {
			return err
		}
		startTime = t
	} else {
		startTime = time.Now()
	}

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

	for _, task := range tasks {
		logGroup, logStream := a.GetLogInfo(task, container)
		nl := strings.Split(logStream, "/")
		prefix := fmt.Sprintf("[%s] ", nl[len(nl)-1])

		if opt.Tail {
			go a.WatchLogs(ctx, logGroup, logStream, startTime, prefix)
		} else {
			a.ShowLogs(ctx, logGroup, logStream, startTime, prefix)
		}
	}

	if opt.Tail {
		select {
		case <-ctx.Done():
			return ctx.Err()
		}
	} else {
		return nil
	}
}
