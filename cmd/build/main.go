// Package main is not intended for use by end users.
//
// It aids in the development of texd, and is otherwise not of much use.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
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
	// prepend help command here, to avoid initialization cycles
	commands = append([]command{{
		name: "help",
		help: "print this help message and exit",
		run: func(args []string) {
			logf("Usage:\n\t%s COMMAND [OPTIONS]", os.Args[0])
			log("\nCommands:")
			for _, cmd := range commands {
				logf("\t%s\t%s\n", cmd.name, cmd.help)
			}
			if len(os.Args) < 1 {
				// main was called without any arguments
				os.Exit(1)
			}
			if len(args) > 0 && args[0] != "help" {
				log()
				runCmd(args[0], []string{"-h"})
			} else {
				logf("\nSee '%s help COMMAND' for details on acceptable options.", os.Args[0])
			}
		},
	}}, commands...)

	if len(os.Args) <= 1 {
		runCmd("help", nil)
	}
	if arg := os.Args[1]; arg == "-h" || arg == "--help" {
		runCmd("help", nil)
	} else {
		runCmd(arg, os.Args[2:])
	}
}

func runCmd(name string, args []string) {
	for _, cmd := range commands {
		if cmd.name == name {
			cmd.run(args)
			os.Exit(0)
		}
	}
	logf("error: command %q not found", name)
	os.Exit(2)
}

type command struct {
	name string
	run  func(args []string)
	help string
}

// list of commands. similar to spf13/cobra, but MUCH simpler
var commands = []command{{
	name: "bump",
	help: "update Git tag",
	run:  cmdBump,
}}

func cmdBump(args []string) {
	fs := pflag.NewFlagSet("bump", pflag.ExitOnError)
	major := fs.BoolP("major", "M", false, "bump major version, reset minor and patch")
	minor := fs.BoolP("minor", "m", false, "bump minor version, reset patch version")
	_ /**/ = fs.BoolP("patch", "p", true, "bump only patch version") // just for the help message
	_ = fs.Parse(args)

	var buf bytes.Buffer
	if err := exec(nil, &buf, "git", "describe", "--tags", "--always", "--dirty"); err != nil {
		fatalf("bump: failed to retrieve tags: %v", err)
	}

	re := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-dirty)?`)
	desc := buf.String()
	m := re.FindStringSubmatch(desc)
	if len(m) == 0 {
		fatalf("bump: no tags, current git description is %q", desc)
	}
	if m[4] != "" {
		log("bump: warning, repo is dirty")
	}

	var maj, min, pat int
	var err error
	if maj, err = strconv.Atoi(m[1]); err != nil {
		fatalf("bump: unexpected major version %q: %v", m[1], err)
	}
	if *major {
		maj++
	} else {
		if min, err = strconv.Atoi(m[2]); err != nil {
			fatalf("bump: unexpected minor version %q: %v", m[2], err)
		}
		if *minor {
			min++
		} else {
			if pat, err = strconv.Atoi(m[3]); err != nil {
				fatalf("bump: unexpected patch version %q: %v", m[3], err)
			}
			pat++
		}
	}

	ver := fmt.Sprintf("v%d.%d.%d", maj, min, pat)
	log("bump:", ver)
	if err := exec(os.Stderr, os.Stdout, "git", "tag", ver); err != nil {
		os.Exit(err.(*osexec.ExitError).ExitCode())
	}
}

func exec(err, out io.Writer, name string, args ...string) error {
	cmd := osexec.Command(name, args...)
	if err != nil {
		cmd.Stderr = err
	} else {
		cmd.Stderr = os.Stderr
	}
	if out != nil {
		cmd.Stdout = out
	} else {
		cmd.Stdout = os.Stdout
	}
	return cmd.Run()
}
