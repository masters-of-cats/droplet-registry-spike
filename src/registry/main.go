package main

import (
	"fmt"
	"os"
	"registry/commands"

	"code.cloudfoundry.org/lager"
	"github.com/urfave/cli"
)

func main() {
	registry := cli.NewApp()
	registry.Name = "registry-experiment"
	registry.Version = "0.0.1"
	registry.Usage = "The Registry Experiment"

	registry.Commands = []cli.Command{
		commands.BuildOCIImage,
	}

	registry.Before = func(ctx *cli.Context) error {
		ctx.App.Metadata["logger"] = createLogger()
		return nil
	}

	if err := registry.Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func createLogger() lager.Logger {
	logger := lager.NewLogger("registry-experiment")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

	return logger
}
