package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Usage = "A ECS deployment tool"

	app.Commands = []*cli.Command{
		deployCommand(),
		runCommand(),
		rollbackCommand(),
		deleteCommand(),
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
