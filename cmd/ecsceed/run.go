package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/maruware/ecsceed"
	"github.com/mattn/go-shellwords"

	"github.com/urfave/cli/v2"
)

func runCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "run",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "service",
				Aliases:  []string{"s"},
				Required: true,
				Usage:    "service name",
			},
			&cli.StringFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Required: true,
				Usage:    "specify config path",
			},
			&cli.StringSliceFlag{
				Name:    "param",
				Aliases: []string{"p"},
				Usage:   "additional params",
			},
			&cli.BoolFlag{
				Name:  "no-wait",
				Usage: "no wait for services stable",
			},
			&cli.Int64Flag{
				Name:  "count",
				Value: 1,
				Usage: "count",
			},
			&cli.StringFlag{
				Name:  "task-def",
				Usage: "task definition",
			},
			&cli.StringFlag{
				Name:  "overrides",
				Usage: "task definition overrides",
			},
			&cli.StringFlag{
				Name:  "command",
				Usage: "execute command",
			},
			&cli.StringFlag{
				Name:  "container",
				Usage: "specify container name",
			},
		},
		Action: func(c *cli.Context) error {
			ctx := context.Background()
			config := c.String("config")
			paramsOpt := c.StringSlice("param")

			params := map[string]string{}
			for _, p := range paramsOpt {
				e := strings.Split(p, "=")
				if len(e) < 2 {
					return fmt.Errorf("Bad param format %s", p)
				}
				params[e[0]] = e[1]
			}

			noWait := c.Bool("no-wait")
			count := c.Int64("count")
			taskDef := c.String("task-def")
			overrides := c.String("overrides")
			commandStr := c.String("command")

			var command []string = nil
			if commandStr != "" {
				var err error
				command, err = shellwords.Parse(commandStr)
				if err != nil {
					return fmt.Errorf("command parse error. %s", commandStr)
				}
			}

			container := c.String("container")

			name := c.String("service")

			app, err := ecsceed.NewApp(config)
			if err != nil {
				return err
			}

			if len(os.Getenv("DEBUG")) > 0 {
				app.Debug = true
			}

			err = app.Run(ctx, name, ecsceed.RunOption{
				AdditionalParams:   params,
				NoWait:             noWait,
				Count:              count,
				TaskDefinitionPath: taskDef,
				Overrides:          overrides,
				Command:            command,
				ContainerName:      container,
			})
			if err != nil {
				return err
			}

			return nil
		},
	}
}
