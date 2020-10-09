package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/maruware/ecsceed"

	"github.com/urfave/cli/v2"
)

func logsCommand() *cli.Command {
	return &cli.Command{
		Name:  "logs",
		Usage: "logs",
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
			&cli.StringFlag{
				Name:  "container",
				Usage: "specify container name",
			},
		},
		Action: func(c *cli.Context) error {
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

			container := c.String("container")

			name := c.String("service")

			app, err := ecsceed.NewApp(config)
			if err != nil {
				return err
			}

			if len(os.Getenv("DEBUG")) > 0 {
				app.Debug = true
			}

			err = app.Logs(c.Context, name, ecsceed.LogsOption{
				AdditionalParams: params,
				ContainerName:    container,
			})
			if err != nil {
				return err
			}

			return nil
		},
	}
}
