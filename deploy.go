package ecsceed

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/fatih/color"
	"github.com/kylelemons/godebug/diff"
)

type DeployOption struct {
	UpdateService      bool
	ForceNewDeployment bool
	AutoLogGroup       bool
	AdditionalParams   Params
	NoWait             bool
	DryRun             bool
}

func (a *App) createLogGroupIfNotExist(ctx context.Context, opt DeployOption) error {
	groups := map[string]struct{}{}

	for _, td := range a.def.nameToTd {
		for _, cd := range td.ContainerDefinitions {
			lc := cd.LogConfiguration
			if lc != nil && *lc.LogDriver == "awslogs" {
				group := lc.Options["awslogs-group"]
				groups[*group] = struct{}{}
			}
		}
	}

	for g := range groups {
		lgs, err := a.DescribeLogGroups(ctx, g)
		if err != nil {
			return err
		}

		isExist := false
		for _, lg := range lgs {
			if *lg.LogGroupName == g {
				isExist = true
				break
			}
		}
		if !isExist {
			if opt.DryRun {
				color.Green("+ CloudWatch Log Group: %s", g)
			} else {
				if err := a.CreateLogGroup(ctx, g); err != nil {
					return err
				}
				a.Log(LogDone(), "Created log group", LogTarget(g))
			}
		}
	}
	return nil
}

func (a *App) createServiceIfNotExist(ctx context.Context, opt DeployOption, srvNames []*string, nameToTdArn map[string]string) error {
	desc, err := a.DescribeServices(ctx, srvNames)
	if err != nil {
		return err
	}

	for _, d := range desc.Failures {
		fullname := arnToName(*d.Arn)

		a.DebugLog("no exist service", fullname)

		name := a.resolveKeyName(fullname)

		srv := a.def.nameToSrv[name]
		srvDef := srv.srv
		srvDef.ServiceName = aws.String(fullname)

		if opt.DryRun {
			color.Yellow("+ service: %s", fullname)
			PrintJSON(srvDef)
		} else {
			tdArn, ok := nameToTdArn[srv.taskDefinition]
			if !ok {
				return fmt.Errorf("Bad reference service to task definition: %s %s", name, srv.taskDefinition)
			}

			err := a.CreateService(ctx, a.def.cluster, tdArn, srvDef)
			if err != nil {
				return err
			}
		}
	}
	for _, d := range desc.Services {
		if *d.Status == "INACTIVE" {
			fullname := *d.ServiceName
			a.DebugLog("INACTIVE service", fullname)

			name := a.resolveKeyName(fullname)

			srv := a.def.nameToSrv[name]
			srvDef := srv.srv
			srvDef.ServiceName = aws.String(fullname)

			if opt.DryRun {
				color.Red("- service: %s", fullname)
				color.Yellow("+ service: %s", fullname)
				PrintJSON(srvDef)
			} else {
				tdArn, ok := nameToTdArn[srv.taskDefinition]
				if !ok {
					return fmt.Errorf("Bad reference service to task definition")
				}

				// once delete
				err := a.DeleteService(ctx, fullname, a.def.cluster, true)
				if err != nil {
					return err
				}

				err = a.CreateService(ctx, a.def.cluster, tdArn, srvDef)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (a *App) updateService(ctx context.Context, opt DeployOption, nameToTdArn map[string]string) error {
	for name, srv := range a.def.nameToSrv {
		fullname := a.resolveFullName(name)

		if opt.DryRun {
			color.Green("~ service with task definition: %s", fullname)
		} else {
			tdArn, ok := nameToTdArn[srv.taskDefinition]
			if !ok {
				return fmt.Errorf("Bad reference service to task definition")
			}

			err := a.UpdateServiceTask(ctx, fullname, tdArn, nil, &opt.ForceNewDeployment)
			if err != nil {
				return err
			}
		}

		if opt.UpdateService {
			if opt.DryRun {
				color.Green("~ service attributes: %s", fullname)
				//TODO: diff

				o, err := a.DescribeServices(ctx, []*string{&fullname})
				if err != nil {
					return err
				}
				curr := o.Services[0]
				d, err := diffService(*curr, srv.srv)
				if err != nil {
					return err
				}

				fmt.Println(d)

				// PrintJSON(srv.srv)
			} else {
				_, err := a.UpdateServiceAttributes(ctx, &srv.srv, fullname, &opt.ForceNewDeployment)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func diffTaskDefinition(a ecs.TaskDefinition, b ecs.TaskDefinition) (string, error) {
	sortTaskDefinitionForDiff(&a)
	sortTaskDefinitionForDiff(&b)

	aBytes, err := MarshalJSON(tdToRegisterTaskDefinitionInput(&a))
	if err != nil {
		return "", err
	}
	bBytes, err := MarshalJSON(tdToRegisterTaskDefinitionInput(&b))
	if err != nil {
		return "", err
	}

	return diff.Diff(string(aBytes), string(bBytes)), nil
}

func diffService(a ecs.Service, b ecs.Service) (string, error) {
	sortServiceDefinitionForDiff(&a)
	sortServiceDefinitionForDiff(&b)

	aBytes, err := MarshalJSON(svToUpdateServiceInput(&a))
	if err != nil {
		return "", err
	}
	bBytes, err := MarshalJSON(svToUpdateServiceInput(&b))
	if err != nil {
		return "", err
	}

	return diff.Diff(string(aBytes), string(bBytes)), nil
}

func svToUpdateServiceInput(sv *ecs.Service) *ecs.UpdateServiceInput {
	return &ecs.UpdateServiceInput{
		CapacityProviderStrategy:      sv.CapacityProviderStrategy,
		DeploymentConfiguration:       sv.DeploymentConfiguration,
		HealthCheckGracePeriodSeconds: sv.HealthCheckGracePeriodSeconds,
		NetworkConfiguration:          sv.NetworkConfiguration,
		PlacementConstraints:          sv.PlacementConstraints,
		PlacementStrategy:             sv.PlacementStrategy,
		PlatformVersion:               sv.PlatformVersion,
	}
}

func (a *App) Deploy(ctx context.Context, opt DeployOption) error {
	err := a.ResolveConfigStack(opt.AdditionalParams)
	if err != nil {
		return err
	}

	nameToTdArn := map[string]string{}
	// register task def
	for name, td := range a.def.nameToTd {
		fullname := a.resolveFullName(name)
		td.SetFamily(fullname)

		if opt.DryRun {
			//TODO: diff
			prevArn, err := a.FindLastTaskDefinition(ctx, fullname)
			if err != nil {
				color.Green("+ task definition: %s", fullname)
				PrintJSON(td)
			} else {
				color.Green("~ task definition: %s", fullname)
				prevTd, err := a.DescribeTaskDefinition(ctx, prevArn)
				if err != nil {
					return err
				}
				d, err := diffTaskDefinition(*prevTd, td)
				if err != nil {
					return err
				}
				fmt.Println(d)
			}
		} else {
			newTd, err := a.RegisterTaskDefinition(ctx, &td)
			if err != nil {
				return err
			}
			tdArn := *newTd.TaskDefinitionArn
			nameToTdArn[name] = tdArn
		}
	}

	// log group
	if opt.AutoLogGroup {
		if err := a.createLogGroupIfNotExist(ctx, opt); err != nil {
			return err
		}
	}

	if len(a.def.nameToSrv) > 0 {
		// create service if not exist
		srvNames := []*string{}
		for name := range a.def.nameToSrv {
			srvNames = append(srvNames, aws.String(a.resolveFullName(name)))
		}

		err = a.createServiceIfNotExist(ctx, opt, srvNames, nameToTdArn)
		if err != nil {
			return err
		}

		// update service
		err = a.updateService(ctx, opt, nameToTdArn)
		if err != nil {
			return err
		}

		if !opt.NoWait && !opt.DryRun {
			now := time.Now()
			if err := a.WaitServiceStable(ctx, now, srvNames); err != nil {
				return err
			}
		}
	}

	if !opt.DryRun {
		a.Log("Deploy Completed!")
	}

	return nil
}
