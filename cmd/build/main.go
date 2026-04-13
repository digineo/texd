// Package main is not intended for use by end users.
//
// It aids in the development of texd, and is otherwise not of much use.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

func log(s ...interface{}) {
	fmt.Fprintln(os.Stderr, s...)
}

func logf(format string, v ...interface{}) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	fmt.Fprintf(os.Stderr, format, v...)
}

func fatalf(format string, v ...interface{}) {
	logf(format, v...)
	os.Exit(1)
}

func main() {
	app := &cli.Command{
		Name:  "build",
		Usage: "development tooling for texd",
		Commands: []*cli.Command{
			{
				Name:  "bump",
				Usage: "update Git tag",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "major",
						Aliases: []string{"M"},
						Usage:   "bump major version, reset minor and patch",
					},
					&cli.BoolFlag{
						Name:    "minor",
						Aliases: []string{"m"},
						Usage:   "bump minor version, reset patch version",
					},
					&cli.BoolFlag{
						Name:    "patch",
						Aliases: []string{"p"},
						Usage:   "bump only patch version",
						Value:   true,
					},
				},
				Action: func(_ context.Context, cmd *cli.Command) error {
					major := cmd.Bool("major")
					minor := cmd.Bool("minor")
					return bumpVersion(major, minor)
				},
			},
			{
				Name:  "docs",
				Usage: "generate HTML documentation from Markdown files",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "input",
						Aliases: []string{"i"},
						Usage:   "Input directory `path` with *.md files",
						Value:   "docs",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output directory `path` to write *.html files to",
						Value:   "service/docs",
					},
					&cli.StringFlag{
						Name:    "readme",
						Aliases: []string{"r"},
						Usage:   "Path to README.md file to update TOC (empty to skip)",
						Value:   "README.md",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					input := cmd.String("input")
					output := cmd.String("output")
					readme := cmd.String("readme")

					return generateDocs(input, output, readme)
				},
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fatalf("error: %v", err)
	}
}
