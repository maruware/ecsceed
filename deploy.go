package ecsceed

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
)

type DeployOption struct {
	UpdateService      bool
	ForceNewDeployment *bool
	AdditionalParams   Params
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

	// create service if not exist
	srvNames := []*string{}
	for name, _ := range a.nameToSrv {
		srvNames = append(srvNames, aws.String(name))
	}
	desc, err := a.DescribeServices(ctx, a.cluster, srvNames)
	if err != nil {
		return err
	}

	for _, d := range desc.Failures {
		name := arnToName(*d.Arn)

		a.DebugLog("no exist service", name)

		srv := a.nameToSrv[name]
		tdArn, ok := nameToTdArn[srv.taskDefinition]
		if !ok {
			return fmt.Errorf("Bad reference service to task definition")
		}

		def := srv.srv
		def.ServiceName = aws.String(name)
		err := a.CreateService(ctx, a.cluster, tdArn, def)
		if err != nil {
			return err
		}
	}
	for _, d := range desc.Services {
		if *d.Status == "INACTIVE" {
			name := *d.ServiceName

			a.DebugLog("INACTIVE service", name)

			srv := a.nameToSrv[name]
			tdArn, ok := nameToTdArn[srv.taskDefinition]
			if !ok {
				return fmt.Errorf("Bad reference service to task definition")
			}

			// once delete
			err := a.DeleteService(ctx, name, a.cluster, true)
			if err != nil {
				return err
			}

			def := srv.srv
			def.ServiceName = aws.String(name)
			err = a.CreateService(ctx, a.cluster, tdArn, def)
			if err != nil {
				return err
			}
		}
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
