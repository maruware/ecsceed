package ecsceed

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
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
var stringerType = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()

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

func formatLogEvent(e *cloudwatchlogs.OutputLogEvent, prefix string, chars int) []string {
	t := time.Unix((*e.Timestamp / int64(1000)), 0)
	line := fmt.Sprintf("%s%s \t%s",
		prefix,
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

func sortSlicesInDefinition(t reflect.Type, v reflect.Value, fieldNames ...string) {
	isSortableField := func(name string) bool {
		for _, n := range fieldNames {
			if n == name {
				return true
			}
		}
		return false
	}
	for i := 0; i < t.NumField(); i++ {
		fv, field := v.Field(i), t.Field(i)
		if fv.Kind() != reflect.Slice || !fv.CanSet() {
			continue
		}
		if !isSortableField(field.Name) {
			continue
		}
		if size := fv.Len(); size == 0 {
			fv.Set(reflect.MakeSlice(fv.Type(), 0, 0))
		} else {
			slice := make([]reflect.Value, size, size)
			for i := 0; i < size; i++ {
				slice[i] = fv.Index(i)
			}
			sort.Slice(slice, func(i, j int) bool {
				iv, jv := reflect.Indirect(slice[i]), reflect.Indirect(slice[j])
				var is, js string
				if iv.Kind() == reflect.String && jv.Kind() == reflect.String {
					is, js = iv.Interface().(string), jv.Interface().(string)
				} else if iv.Type().Implements(stringerType) && jv.Type().Implements(stringerType) {
					is, js = iv.Interface().(fmt.Stringer).String(), jv.Interface().(fmt.Stringer).String()
				}
				return is < js
			})
			sorted := reflect.MakeSlice(fv.Type(), size, size)
			for i := 0; i < size; i++ {
				sorted.Index(i).Set(slice[i])
			}
			fv.Set(sorted)
		}
	}
}

func equalString(a *string, b string) bool {
	if a == nil {
		return b == ""
	}
	return *a == b
}

func sortServiceDefinitionForDiff(sv *ecs.Service) {
	sortSlicesInDefinition(
		reflect.TypeOf(*sv), reflect.Indirect(reflect.ValueOf(sv)),
		"PlacementConstraints",
		"PlacementStrategy",
		"RequiresCompatibilities",
	)
	if equalString(sv.LaunchType, ecs.LaunchTypeFargate) && sv.PlatformVersion == nil {
		sv.PlatformVersion = aws.String("LATEST")
	}
	if sv.SchedulingStrategy == nil && sv.DeploymentConfiguration == nil {
		sv.DeploymentConfiguration = &ecs.DeploymentConfiguration{
			MaximumPercent:        aws.Int64(200),
			MinimumHealthyPercent: aws.Int64(100),
		}
	} else if equalString(sv.SchedulingStrategy, ecs.SchedulingStrategyDaemon) && sv.DeploymentConfiguration == nil {
		sv.DeploymentConfiguration = &ecs.DeploymentConfiguration{
			MaximumPercent:        aws.Int64(100),
			MinimumHealthyPercent: aws.Int64(0),
		}
	}

	if len(sv.LoadBalancers) > 0 && sv.HealthCheckGracePeriodSeconds == nil {
		sv.HealthCheckGracePeriodSeconds = aws.Int64(0)
	}
	if nc := sv.NetworkConfiguration; nc != nil {
		if ac := nc.AwsvpcConfiguration; ac != nil {
			if ac.AssignPublicIp == nil {
				ac.AssignPublicIp = aws.String(ecs.AssignPublicIpDisabled)
			}
			sortSlicesInDefinition(
				reflect.TypeOf(*ac),
				reflect.Indirect(reflect.ValueOf(ac)),
				"SecurityGroups",
				"Subnets",
			)
		}
	}
}

func sortTaskDefinitionForDiff(td *ecs.TaskDefinition) {
	for _, cd := range td.ContainerDefinitions {
		if cd.Cpu == nil {
			cd.Cpu = aws.Int64(0)
		}
		sortSlicesInDefinition(
			reflect.TypeOf(*cd), reflect.Indirect(reflect.ValueOf(cd)),
			"Environment",
			"MountPoints",
			"PortMappings",
			"VolumesFrom",
			"Secrets",
		)
	}
	sortSlicesInDefinition(
		reflect.TypeOf(*td), reflect.Indirect(reflect.ValueOf(td)),
		"ContainerDefinitions",
		"PlacementConstraints",
		"RequiresCompatibilities",
		"Volumes",
	)
	if td.Cpu != nil {
		td.Cpu = toNumberCPU(*td.Cpu)
	}
	if td.Memory != nil {
		td.Memory = toNumberMemory(*td.Memory)
	}
}

func toNumberCPU(cpu string) *string {
	if i := strings.Index(strings.ToLower(cpu), "vcpu"); i > 0 {
		if ns, err := strconv.ParseFloat(strings.Trim(cpu[0:i], " "), 64); err != nil {
			return nil
		} else {
			nn := fmt.Sprintf("%d", int(ns*1024))
			return &nn
		}
	}
	return &cpu
}

func toNumberMemory(memory string) *string {
	if i := strings.Index(memory, "GB"); i > 0 {
		if ns, err := strconv.ParseFloat(strings.Trim(memory[0:i], " "), 64); err != nil {
			return nil
		} else {
			nn := fmt.Sprintf("%d", int(ns*1024))
			return &nn
		}
	}
	return &memory
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
	a.Log(LogDone(), "Registered task definition", LogTarget(name))
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
		return fmt.Errorf("Failed to update service task: %w", err)
	}
	time.Sleep(delayForServiceChanged) // wait for service updated

	a.Log(LogDone(), "Update service task definition",
		LogTarget(name), "with", LogTarget(arnToName(taskDefinitionArn)))
	return nil
}

func (a *App) DescribeServices(ctx context.Context, names []*string) (*ecs.DescribeServicesOutput, error) {
	return a.ecs.DescribeServicesWithContext(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(a.def.cluster),
		Services: names,
	})
}

func (a *App) DescribeService(ctx context.Context, name *string) (*ecs.Service, error) {
	o, err := a.DescribeServices(ctx, []*string{name})
	if err != nil {
		return nil, err
	}
	if len(o.Services) == 0 {
		return nil, fmt.Errorf("not found service %s", *name)
	}
	return o.Services[0], nil
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
		return errors.Wrap(err, "Failed to create service")
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
	a.Log("Ran task", LogTarget(*task.TaskArn))
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

func getLogEventsInput(logGroup string, logStream string, startAt int64) *cloudwatchlogs.GetLogEventsInput {
	return &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		StartTime:     aws.Int64(startAt),
	}
}

func (a *App) PrintLogEvents(ctx context.Context, logGroup string, logStream string, startedAt time.Time, prefix string) (int, error) {
	ms := startedAt.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
	out, err := a.cwl.GetLogEventsWithContext(ctx, getLogEventsInput(logGroup, logStream, ms))
	if err != nil {
		return 0, err
	}

	if len(out.Events) == 0 {
		return 0, nil
	}
	var lines int
	for _, event := range out.Events {
		for _, line := range formatLogEvent(event, prefix, TerminalWidth) {
			fmt.Println(line)
			lines++
		}
	}

	return lines, nil
}

func (a *App) WatchLogs(ctx context.Context, logGroup, logStream string, startedAt time.Time, prefix string) error {
	tick := time.Tick(5 * time.Second)

	ms := startedAt.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick:
			o, err := a.cwl.GetLogEventsWithContext(ctx, getLogEventsInput(logGroup, logStream, ms))
			if err != nil {
				return err
			}
			for _, ev := range o.Events {
				for _, line := range formatLogEvent(ev, prefix, TerminalWidth) {
					fmt.Println(line)
				}
			}
			if len(o.Events) > 0 {
				ms = *o.Events[len(o.Events)-1].Timestamp + 1
			}
		}
	}
}

func (a *App) ShowLogs(ctx context.Context, logGroup, logStream string, startedAt time.Time, prefix string) {
	a.PrintLogEvents(ctx, logGroup, logStream, startedAt, prefix)
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
		a.WatchLogs(waitCtx, logGroup, logStream, startedAt, "")
	}()

	if err := a.WaitUntilTaskStopped(ctx, task); err != nil {
		return errors.Wrap(err, "Failed to run task")
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

func (a *App) DescribeTaskStatus(ctx context.Context, task *ecs.Task, watchContainer *ecs.ContainerDefinition) error {
	out, err := a.ecs.DescribeTasksWithContext(ctx, a.DescribeTasksInput(task))
	if err != nil {
		return err
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		return fmt.Errorf("Failed to describe task %s: %s", *f.Arn, *f.Reason)
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

func (a *App) DeregisterTaskDefinition(ctx context.Context, tdArn string) error {
	_, err := a.ecs.DeregisterTaskDefinitionWithContext(
		ctx,
		&ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String(tdArn),
		},
	)
	return err
}

func (a *App) FindRollbackTarget(ctx context.Context, tdArn string) (string, error) {
	var found bool
	var nextToken *string
	family := strings.Split(arnToName(tdArn), ":")[0]
	for {
		out, err := a.ecs.ListTaskDefinitionsWithContext(ctx,
			&ecs.ListTaskDefinitionsInput{
				NextToken:    nextToken,
				FamilyPrefix: aws.String(family),
				MaxResults:   aws.Int64(100),
				Sort:         aws.String("DESC"),
			},
		)
		if err != nil {
			return "", errors.Wrap(err, "Failed to list taskdefinitions")
		}
		if len(out.TaskDefinitionArns) == 0 {
			return "", errors.New("Rollback target is not found")
		}
		nextToken = out.NextToken
		for _, t := range out.TaskDefinitionArns {
			if found {
				return *t, nil
			}
			if *t == tdArn {
				found = true
			}
		}
	}
}

func (a *App) FindLastTaskDefinition(ctx context.Context, tdName string) (string, error) {
	family := strings.Split(tdName, ":")[0]
	for {
		out, err := a.ecs.ListTaskDefinitionsWithContext(ctx,
			&ecs.ListTaskDefinitionsInput{
				NextToken:    nil,
				FamilyPrefix: aws.String(family),
				MaxResults:   aws.Int64(100),
				Sort:         aws.String("DESC"),
			},
		)
		if err != nil {
			return "", errors.Wrap(err, "Failed to list taskdefinitions")
		}
		if len(out.TaskDefinitionArns) == 0 {
			return "", errors.New("Rollback target is not found")
		}
		return *out.TaskDefinitionArns[0], nil
	}
}

func (a *App) DescribeCluster(ctx context.Context, nameOrArn string) (*ecs.Cluster, error) {
	out, err := a.ecs.DescribeClustersWithContext(ctx, &ecs.DescribeClustersInput{
		Clusters: []*string{aws.String(nameOrArn)},
	})
	if err != nil {
		return nil, err
	}

	if len(out.Clusters) == 0 {
		return nil, nil
	}
	return out.Clusters[0], nil
}

func (a *App) ListServiceTasks(ctx context.Context, name string) ([]*ecs.Task, error) {
	var nextToken *string

	tasks := []*string{}
	for {
		out, err := a.ecs.ListTasksWithContext(ctx, &ecs.ListTasksInput{
			Cluster:     aws.String(a.def.cluster),
			ServiceName: aws.String(name),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, out.TaskArns...)

		if out.NextToken != nil {
			nextToken = out.NextToken
		} else {
			break
		}
	}

	out, err := a.ecs.DescribeTasksWithContext(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(a.def.cluster),
		Tasks:   tasks,
	})
	if err != nil {
		return nil, err
	}

	return out.Tasks, nil
}

func (a *App) DescribeContainerInstance(ctx context.Context, arn string) (*ecs.ContainerInstance, error) {
	out, err := a.ecs.DescribeContainerInstancesWithContext(ctx, &ecs.DescribeContainerInstancesInput{
		Cluster:            aws.String(a.def.cluster),
		ContainerInstances: []*string{aws.String(arn)},
	})
	if err != nil {
		return nil, err
	}
	if len(out.ContainerInstances) == 0 {
		return nil, nil
	}
	return out.ContainerInstances[0], nil
}
