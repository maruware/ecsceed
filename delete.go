package ecsceed

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/fatih/color"
)

type DeleteOption struct {
	DryRun bool
}

func (a *App) Delete(ctx context.Context, opt DeleteOption) error {
	err := a.ResolveConfigStack(Params{})
	if err != nil {
		return err
	}
	srvNames := []*string{}
	for name := range a.def.nameToSrv {
		srvNames = append(srvNames, aws.String(a.resolveFullName(name)))
	}
	a.Log(srvNames)
	desc, err := a.DescribeServices(ctx, srvNames)
	if err != nil {
		return err
	}

	for _, s := range desc.Services {
		if opt.DryRun {
			color.Red("- service: %s", *s.ServiceName)
		} else {
			input := &ecs.DeleteServiceInput{
				Cluster: s.ClusterArn,
				Service: s.ServiceName,
			}
			if _, err := a.ecs.DeleteServiceWithContext(ctx, input); err != nil {
				return fmt.Errorf("Failed to delete service: %w", err)
			}
			a.Log("Service is deleted")
		}
	}

	return nil
}
