package main

import (
	"context"
	"os"

	"github.com/maruware/ecsceed"

	"github.com/urfave/cli/v2"
)

func rollbackCommand() *cli.Command {
	return &cli.Command{
		Name:  "rollback",
		Usage: "rollback",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Required: true,
				Usage:    "specify config path",
			},
			&cli.BoolFlag{
				Name:  "no-wait",
				Usage: "no wait for services stable",
			},
			&cli.StringFlag{
				Name:  "dry-run",
				Usage: "dry run",
			},
			&cli.StringFlag{
				Name:  "deregister",
				Usage: "deregister task definition",
			},
		},
		Action: func(c *cli.Context) error {
			ctx := context.Background()
			config := c.String("config")

			noWait := c.Bool("no-wait")
			dryRun := c.Bool("dry-run")
			deregister := c.Bool("deregister")

			app, err := ecsceed.NewApp(config)
			if err != nil {
				return err
			}

			if len(os.Getenv("DEBUG")) > 0 {
				app.Debug = true
			}

			err = app.Rollback(ctx, ecsceed.RollbackOption{
				NoWait:                   noWait,
				DryRun:                   dryRun,
				DeregisterTaskDefinition: deregister,
			})
			if err != nil {
				return err
			}

			return nil
		},
	}
}
