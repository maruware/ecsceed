package main

import (
	"context"
	"os"

	"github.com/maruware/ecsceed"

	"github.com/urfave/cli/v2"
)

func deleteCommand() *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "delete",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Required: true,
				Usage:    "specify config path",
			},
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "dry run",
			},
		},
		Action: func(c *cli.Context) error {
			ctx := context.Background()
			config := c.String("config")
			dryRun := c.Bool("dry-run")

			app, err := ecsceed.NewApp(config)
			if err != nil {
				return err
			}

			if len(os.Getenv("DEBUG")) > 0 {
				app.Debug = true
			}

			err = app.Delete(ctx, ecsceed.DeleteOption{
				DryRun: dryRun,
			})
			if err != nil {
				return err
			}

			return nil
		},
	}
}
