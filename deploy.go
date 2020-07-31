package ecsceed

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
)

type DeployOption struct {
	UpdateService      bool
	ForceNewDeployment *bool
	AdditionalParams   Params
	NoWait             bool
}

func (a *App) Deploy(ctx context.Context, opt DeployOption) error {
	err := a.ResolveConfigStack(opt.AdditionalParams)
	if err != nil {
		return err
	}

	nameToTdArn := map[string]string{}
	// register task def
	for name, td := range a.def.nameToTd {
		td.SetFamily(a.resolveFullName(name))
		newTd, err := a.RegisterTaskDefinition(ctx, &td)
		if err != nil {
			return err
		}
		tdArn := *newTd.TaskDefinitionArn
		nameToTdArn[name] = tdArn
	}

	// create service if not exist
	srvNames := []*string{}
	for name := range a.def.nameToSrv {
		srvNames = append(srvNames, aws.String(a.resolveFullName(name)))
	}
	desc, err := a.DescribeServices(ctx, srvNames)
	if err != nil {
		return err
	}

	for _, d := range desc.Failures {
		fullname := arnToName(*d.Arn)

		a.DebugLog("no exist service", fullname)

		name := a.resolveKeyName(fullname)

		srv := a.def.nameToSrv[name]
		tdArn, ok := nameToTdArn[srv.taskDefinition]
		if !ok {
			return fmt.Errorf("Bad reference service to task definition")
		}

		def := srv.srv
		def.ServiceName = aws.String(fullname)
		err := a.CreateService(ctx, a.def.cluster, tdArn, def)
		if err != nil {
			return err
		}
	}
	for _, d := range desc.Services {
		if *d.Status == "INACTIVE" {
			fullname := *d.ServiceName
			a.DebugLog("INACTIVE service", fullname)

			name := a.resolveKeyName(fullname)

			srv := a.def.nameToSrv[name]
			tdArn, ok := nameToTdArn[srv.taskDefinition]
			if !ok {
				return fmt.Errorf("Bad reference service to task definition")
			}

			// once delete
			err := a.DeleteService(ctx, fullname, a.def.cluster, true)
			if err != nil {
				return err
			}

			def := srv.srv
			def.ServiceName = aws.String(fullname)
			err = a.CreateService(ctx, a.def.cluster, tdArn, def)
			if err != nil {
				return err
			}
		}
	}

	// update service
	for name, srv := range a.def.nameToSrv {

		tdArn, ok := nameToTdArn[srv.taskDefinition]
		if !ok {
			return fmt.Errorf("Bad reference service to task definition")
		}

		fullname := a.resolveFullName(name)

		err := a.UpdateServiceTask(ctx, fullname, tdArn, nil, opt.ForceNewDeployment)
		if err != nil {
			return err
		}
		if opt.UpdateService {
			_, err := a.UpdateServiceAttributes(ctx, &srv.srv, fullname, opt.ForceNewDeployment)
			if err != nil {
				return err
			}
		}
	}

	if !opt.NoWait {
		now := time.Now()
		if err := a.WaitServiceStable(ctx, now, srvNames); err != nil {
			return err
		}
	}

	a.Log("Finish!")

	return nil
}
