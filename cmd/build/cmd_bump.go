package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"regexp"
	"strconv"
)

func bumpVersion(major, minor bool) error {
	var buf bytes.Buffer
	if err := exec(nil, &buf, "git", "describe", "--tags", "--always", "--dirty"); err != nil {
		return fmt.Errorf("bump: failed to retrieve tags: %v", err)
	}

	re := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-dirty)?`)
	desc := buf.String()
	m := re.FindStringSubmatch(desc)
	if len(m) == 0 {
		return fmt.Errorf("bump: no tags, current git description is %q", desc)
	}
	if m[4] != "" {
		log("bump: warning, repo is dirty")
	}

	var maj, min, pat int
	var err error
	if maj, err = strconv.Atoi(m[1]); err != nil {
		return fmt.Errorf("bump: invalid major version %q: %v", m[1], err)
	}
	if min, err = strconv.Atoi(m[2]); err != nil {
		return fmt.Errorf("bump: invalid minor version %q: %v", m[2], err)
	}
	if pat, err = strconv.Atoi(m[3]); err != nil {
		return fmt.Errorf("bump: invalid patch version %q: %v", m[3], err)
	}

	if major {
		maj++
	} else if minor {
		min++
	} else {
		pat++
	}

	ver := fmt.Sprintf("v%d.%d.%d", maj, min, pat)
	log("bump:", ver)
	if err := exec(os.Stderr, os.Stdout, "git", "tag", ver); err != nil {
		return fmt.Errorf("bump: failed to bump version: %v", err)
	}
	return nil
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
