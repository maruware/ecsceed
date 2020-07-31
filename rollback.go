package ecsceed

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/fatih/color"
	"github.com/pkg/errors"
)

type RollbackOption struct {
	NoWait                   bool
	ForceNewDeployment       bool
	DeregisterTaskDefinition bool
	DryRun                   bool
}

func (a *App) Rollback(ctx context.Context, opt RollbackOption) error {
	a.Log("Starting rollback")

	err := a.ResolveConfigStack(Params{})
	if err != nil {
		return err
	}
	srvNames := []*string{}
	for name := range a.def.nameToSrv {
		srvNames = append(srvNames, aws.String(a.resolveFullName(name)))
	}
	desc, err := a.DescribeServices(ctx, srvNames)
	if err != nil {
		return err
	}

	for _, s := range desc.Services {
		currentArn := *s.TaskDefinition
		targetArn, err := a.FindRollbackTarget(ctx, currentArn)
		if err != nil {
			return errors.Wrap(err, "failed to find rollback target")
		}

		fullname := *s.ServiceName

		if opt.DryRun {
			color.YellowString("~ rollback task definition: service=%s task definition=%s", fullname, arnToName(targetArn))
		} else {
			f := false // Set ForceNewDeployment and UpdateService to false
			a.Log("rollbacking", LogTarget(arnToName(currentArn)), "->", LogTarget(arnToName(targetArn)))
			if err := a.UpdateServiceTask(
				ctx,
				fullname,
				targetArn,
				s.DesiredCount,
				&f,
			); err != nil {
				return errors.Wrap(err, "failed to update service")
			}
		}
	}

	if opt.NoWait {
		a.Log("Service is rollbacked.")
		return nil
	}
	time.Sleep(delayForServiceChanged) // wait for service updated

	if !opt.DryRun {
		if err := a.WaitServiceStable(ctx, time.Now(), srvNames); err != nil {
			return errors.Wrap(err, "failed to wait service stable")
		}
		a.Log("Service is stable now. Completed!")
	}

	if opt.DeregisterTaskDefinition {
		for _, s := range desc.Services {
			currentArn := *s.TaskDefinition

			if opt.DryRun {
				a.Log("- task definition", arnToName(currentArn))
			} else {
				a.Log("Deregistering rolled back task definition", arnToName(currentArn))
				err := a.DeregisterTaskDefinition(ctx, currentArn)
				if err != nil {
					return fmt.Errorf("failed to deregister task definition: %w", err)
				}
				a.Log(arnToName(currentArn), "was deregistered successfully")
			}
		}
	}

	return nil
}
