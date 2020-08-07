package main

import (
	"os"

	"github.com/maruware/ecsceed"

	"github.com/urfave/cli/v2"
)

func statusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "status",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Required: true,
				Usage:    "specify config path",
			},
			&cli.IntFlag{
				Name:  "events",
				Usage: "display events num",
				Value: 3,
			},
		},
		Action: func(c *cli.Context) error {
			config := c.String("config")

			events := c.Int("events")

			app, err := ecsceed.NewApp(config)
			if err != nil {
				return err
			}

			if len(os.Getenv("DEBUG")) > 0 {
				app.Debug = true
			}

			err = app.Status(c.Context, ecsceed.StatusOption{
				Events: events,
			})
			if err != nil {
				return err
			}

			return nil
		},
	}
}
