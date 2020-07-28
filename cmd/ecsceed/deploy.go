package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/maruware/ecsceed"

	"github.com/urfave/cli/v2"
)

func deployCommand() *cli.Command {
	return &cli.Command{
		Name:  "deploy",
		Usage: "deploy",
		Flags: []cli.Flag{
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

			app, err := ecsceed.NewApp(config)
			if err != nil {
				return err
			}

			err = app.Deploy(ctx, ecsceed.DeployOption{
				AdditionalParams: params,
			})
			if err != nil {
				return err
			}

			return nil
		},
	}
}
