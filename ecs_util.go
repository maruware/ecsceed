package ecsceed

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/mattn/go-isatty"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
)

var isTerminal = isatty.IsTerminal(os.Stdout.Fd())
var delayForServiceChanged = 3 * time.Second
var TerminalWidth = 120

var timezone, _ = time.LoadLocation("Local")

func arnToName(s string) string {
	ns := strings.Split(s, "/")
	return ns[len(ns)-1]
}

func formatDeployment(d *ecs.Deployment) string {
	return fmt.Sprintf(
		"%8s %s desired:%d pending:%d running:%d",
		*d.Status,
		LogTarget(arnToName(*d.TaskDefinition)),
		*d.DesiredCount, *d.PendingCount, *d.RunningCount,
	)
}

func formatEvent(e *ecs.ServiceEvent, chars int) []string {
	line := fmt.Sprintf("%s \t%s",
		e.CreatedAt.In(timezone).Format("2006/01/02 15:04:05"),
		*e.Message,
	)
	lines := []string{}
	n := len(line)/chars + 1
	for i := 0; i < n; i++ {
		if i == n-1 {
			lines = append(lines, line[i*chars:])
		} else {
			lines = append(lines, line[i*chars:(i+1)*chars])
		}
	}
	return lines
}

func formatLogEvent(e *cloudwatchlogs.OutputLogEvent, chars int) []string {
	t := time.Unix((*e.Timestamp / int64(1000)), 0)
	line := fmt.Sprintf("%s \t%s",
		t.In(timezone).Format("2006/01/02 15:04:05"),
		*e.Message,
	)
	lines := []string{}
	n := len(line)/chars + 1
	for i := 0; i < n; i++ {
		if i == n-1 {
			lines = append(lines, line[i*chars:])
		} else {
			lines = append(lines, line[i*chars:(i+1)*chars])
		}
	}
	return lines
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

	name := arnToName(*out.TaskDefinition.TaskDefinitionArn)
	a.Log(LogDone(), "registered task definition", LogTarget(name))
	return out.TaskDefinition, nil
}

func (a *App) UpdateServiceAttributes(ctx context.Context, srv *ecs.Service, name string, forceNewDeployment *bool) (*ecs.Service, error) {
	in := srvToUpdateServiceInput(srv)
	in.ForceNewDeployment = forceNewDeployment

	in.Service = aws.String(name)
	in.Cluster = aws.String(a.def.cluster)

	out, err := a.ecs.UpdateServiceWithContext(ctx, in)
	if err != nil {
		return nil, err
	}
	time.Sleep(delayForServiceChanged) // wait for service updated
	sv := out.Service

	return sv, nil
}

func (a *App) UpdateServiceTask(ctx context.Context, name string, taskDefinitionArn string, count *int64, forceNewDeployment *bool) error {
	in := &ecs.UpdateServiceInput{
		Service:            aws.String(name),
		Cluster:            aws.String(a.def.cluster),
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
		return fmt.Errorf("failed to update service task: %w", err)
	}
	time.Sleep(delayForServiceChanged) // wait for service updated

	a.Log(LogDone(), "update service task definition",
		LogTarget(name), "with", LogTarget(arnToName(taskDefinitionArn)))
	return nil
}

func (a *App) DescribeServices(ctx context.Context, names []*string) (*ecs.DescribeServicesOutput, error) {
	return a.ecs.DescribeServicesWithContext(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(a.def.cluster),
		Services: names,
	})
}

func (a *App) CreateService(ctx context.Context, cluster string, tdArn string, srv ecs.Service) error {
	a.Log("Starting create service", *srv.ServiceName)

	createServiceInput := &ecs.CreateServiceInput{
		Cluster:                       aws.String(cluster),
		CapacityProviderStrategy:      srv.CapacityProviderStrategy,
		DeploymentConfiguration:       srv.DeploymentConfiguration,
		DeploymentController:          srv.DeploymentController,
		DesiredCount:                  srv.DesiredCount,
		EnableECSManagedTags:          srv.EnableECSManagedTags,
		HealthCheckGracePeriodSeconds: srv.HealthCheckGracePeriodSeconds,
		LaunchType:                    srv.LaunchType,
		LoadBalancers:                 srv.LoadBalancers,
		NetworkConfiguration:          srv.NetworkConfiguration,
		PlacementConstraints:          srv.PlacementConstraints,
		PlacementStrategy:             srv.PlacementStrategy,
		PlatformVersion:               srv.PlatformVersion,
		PropagateTags:                 srv.PropagateTags,
		SchedulingStrategy:            srv.SchedulingStrategy,
		ServiceName:                   srv.ServiceName,
		ServiceRegistries:             srv.ServiceRegistries,
		Tags:                          srv.Tags,
		TaskDefinition:                aws.String(tdArn),
	}
	if _, err := a.ecs.CreateServiceWithContext(ctx, createServiceInput); err != nil {
		return errors.Wrap(err, "failed to create service")
	}

	time.Sleep(delayForServiceChanged) // wait for service updated

	a.Log("Service is created", *srv.ServiceName)

	return nil
}

func (a *App) DeleteService(ctx context.Context, name string, cluster string, force bool) error {
	out, err := a.ecs.DeleteServiceWithContext(ctx, &ecs.DeleteServiceInput{
		Cluster: aws.String(cluster),
		Force:   aws.Bool(force),
		Service: aws.String(name),
	})

	if err != nil {
		return err
	}

	a.Log("Service is deleted", *out.Service.ServiceName)
	return nil
}

func (a *App) DescribeServiceDeployments(ctx context.Context, startedAt time.Time, names []*string) (int, error) {
	out, err := a.DescribeServices(ctx, names)
	if err != nil {
		return 0, err
	}
	if len(out.Services) == 0 {
		return 0, nil
	}

	lines := 0
	for _, s := range out.Services {
		for _, dep := range s.Deployments {
			lines++
			a.Log(formatDeployment(dep))
		}
		for _, event := range s.Events {
			if (*event.CreatedAt).After(startedAt) {
				for _, line := range formatEvent(event, TerminalWidth) {
					fmt.Println(line)
					lines++
				}
			}
		}
	}

	return lines, nil
}

func (a *App) WaitServiceStable(ctx context.Context, startedAt time.Time, names []*string) error {
	a.Log("Waiting for service stable...(it will take a few minutes)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		tick := time.Tick(10 * time.Second)
		var lines int
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-tick:
				if isTerminal {
					for i := 0; i < lines; i++ {
						fmt.Print(aec.EraseLine(aec.EraseModes.All), aec.PreviousLine(1))
					}
				}
				lines, _ = a.DescribeServiceDeployments(waitCtx, startedAt, names)
			}
		}
	}()

	// // Add an option WithWaiterDelay and request.WithWaiterMaxAttempts for a long timeout.
	// // SDK Default is 10 min (MaxAttempts=40 * Delay=15sec) at now.
	// // ref. https://github.com/aws/aws-sdk-go/blob/d57c8d96f72d9475194ccf18d2ba70ac294b0cb3/service/ecs/waiters.go#L82-L83
	// // Explicitly set these options so not being affected by the default setting.
	// const delay = 15 * time.Second
	// attempts := int((a.config.Timeout / delay)) + 1
	// if (a.config.Timeout % delay) > 0 {
	// 	attempts++
	// }
	return a.ecs.WaitUntilServicesStableWithContext(
		ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(a.def.cluster),
			Services: names,
		},
	)
}

func (a *App) DescribeTaskDefinition(ctx context.Context, tdArn string) (*ecs.TaskDefinition, error) {
	out, err := a.ecs.DescribeTaskDefinitionWithContext(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &tdArn,
	})
	if err != nil {
		return nil, err
	}
	return out.TaskDefinition, nil
}

func (a *App) RunTask(ctx context.Context, srv ecs.Service, tdArn string, count int64, ov *ecs.TaskOverride) (*ecs.Task, error) {
	out, err := a.ecs.RunTaskWithContext(ctx, &ecs.RunTaskInput{
		CapacityProviderStrategy: srv.CapacityProviderStrategy,
		Cluster:                  srv.ClusterArn,
		Count:                    aws.Int64(count),
		LaunchType:               srv.LaunchType,
		NetworkConfiguration:     srv.NetworkConfiguration,
		PlacementConstraints:     srv.PlacementConstraints,
		PlacementStrategy:        srv.PlacementStrategy,
		PlatformVersion:          srv.PlatformVersion,
		TaskDefinition:           aws.String(tdArn),
		Overrides:                ov,
	})
	if err != nil {
		return nil, err
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		return nil, errors.New(*f.Reason)
	}

	task := out.Tasks[0]
	a.Log("ran task", LogTarget(*task.TaskArn))
	return task, nil
}

func (a *App) GetLogInfo(task *ecs.Task, c *ecs.ContainerDefinition) (string, string) {
	p := strings.Split(*task.TaskArn, "/")
	taskID := p[len(p)-1]
	lc := c.LogConfiguration
	logStreamPrefix := *lc.Options["awslogs-stream-prefix"]

	logStream := strings.Join([]string{logStreamPrefix, *c.Name, taskID}, "/")
	logGroup := *lc.Options["awslogs-group"]

	a.Log("logGroup:", logGroup)
	a.Log("logStream:", logStream)

	return logGroup, logStream
}

func (a *App) GetLogEventsInput(logGroup string, logStream string, startAt int64) *cloudwatchlogs.GetLogEventsInput {
	return &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		StartTime:     aws.Int64(startAt),
	}
}

func (a *App) GetLogEvents(ctx context.Context, logGroup string, logStream string, startedAt time.Time) (int, error) {
	ms := startedAt.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
	out, err := a.cwl.GetLogEventsWithContext(ctx, a.GetLogEventsInput(logGroup, logStream, ms))
	if err != nil {
		return 0, err
	}
	if len(out.Events) == 0 {
		return 0, nil
	}
	lines := 0
	for _, event := range out.Events {
		for _, line := range formatLogEvent(event, TerminalWidth) {
			fmt.Println(line)
			lines++
		}
	}
	return lines, nil
}

func (a *App) WaitRunTask(ctx context.Context, task *ecs.Task, watchContainer *ecs.ContainerDefinition, startedAt time.Time) error {
	a.Log("Waiting for run task...(it may take a while)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	lc := watchContainer.LogConfiguration
	if lc == nil || *lc.LogDriver != "awslogs" || lc.Options["awslogs-stream-prefix"] == nil {
		a.Log("awslogs not configured")
		if err := a.WaitUntilTaskStopped(ctx, task); err != nil {
			return errors.Wrap(err, "failed to run task")
		}
		return nil
	}

	logGroup, logStream := a.GetLogInfo(task, watchContainer)
	time.Sleep(3 * time.Second) // wait for log stream

	go func() {
		tick := time.Tick(5 * time.Second)
		var lines int
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-tick:
				if isTerminal {
					for i := 0; i < lines; i++ {
						fmt.Print(aec.EraseLine(aec.EraseModes.All), aec.PreviousLine(1))
					}
				}
				lines, _ = a.GetLogEvents(waitCtx, logGroup, logStream, startedAt)
			}
		}
	}()

	if err := a.WaitUntilTaskStopped(ctx, task); err != nil {
		return errors.Wrap(err, "failed to run task")
	}
	return nil
}

func (a *App) WaitUntilTaskStopped(ctx context.Context, task *ecs.Task) error {
	return a.ecs.WaitUntilTasksStoppedWithContext(
		ctx, a.DescribeTasksInput(task),
	)
}

func (a *App) DescribeTasksInput(task *ecs.Task) *ecs.DescribeTasksInput {
	return &ecs.DescribeTasksInput{
		Cluster: aws.String(a.def.cluster),
		Tasks:   []*string{task.TaskArn},
	}
}

func (d *App) DescribeTaskStatus(ctx context.Context, task *ecs.Task, watchContainer *ecs.ContainerDefinition) error {
	out, err := d.ecs.DescribeTasksWithContext(ctx, d.DescribeTasksInput(task))
	if err != nil {
		return err
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		d.Log("Task ARN: " + *f.Arn)
		return errors.New(*f.Reason)
	}

	var container *ecs.Container
	for _, c := range out.Tasks[0].Containers {
		if *c.Name == *watchContainer.Name {
			container = c
			break
		}
	}
	if container == nil {
		container = out.Tasks[0].Containers[0]
	}

	if container.ExitCode != nil && *container.ExitCode != 0 {
		msg := fmt.Sprintf("Container: %s, Exit Code: %s", *container.Name, strconv.FormatInt(*container.ExitCode, 10))
		if container.Reason != nil {
			msg += ", Reason: " + *container.Reason
		}
		return errors.New(msg)
	} else if container.Reason != nil {
		return fmt.Errorf("Container: %s, Reason: %s", *container.Name, *container.Reason)
	}
	return nil
}
