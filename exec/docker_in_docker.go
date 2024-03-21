package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/mount"
	"github.com/spf13/afero"
)

// baseDirRewrite is used to rewrite the work directory when executing
// jobs (this is for Docker-in-Docker setups).
type baseDirRewrite struct {
	src string // bind or volume path (on host)
	dst string // baseDir (in this container)
}

func (r *baseDirRewrite) MountConfig(path string) mount.Mount {
	if r != nil {
		path = filepath.Join(r.src, strings.TrimPrefix(path, r.dst))
	}
	return mount.Mount{
		Type:   mount.TypeBind,
		Source: path,
		Target: containerWd,
	}
}

var ErrMissingWorkdirVolume = errors.New("missing Docker volume or bind mount for work directory")

// swapped in tests.
var (
	dockerFs = afero.NewOsFs()
	hostname = os.Hostname
)

func (dc *DockerClient) configureDinD(baseDir string) error {
	if _, err := dockerFs.Stat("/.dockerenv"); errors.Is(err, os.ErrNotExist) {
		return nil // we're not running inside a container
	}
	if baseDir == "" {
		return nil // no --job-directory given, assuming no DIND setup
	}

	id, err := determineContainerID()
	if err != nil {
		return fmt.Errorf("cannot determine texd container ID: %w", err)
	}
	container, err := dc.cli.ContainerInspect(context.Background(), id)
	if err != nil {
		return fmt.Errorf("cannot determine texd container: %w", err)
	}

	for _, mp := range container.Mounts {
		if mp.Destination != baseDir {
			continue
		}

		switch mp.Type {
		case mount.TypeVolume:
			if mp.Driver != "local" {
				return fmt.Errorf("%s volume binds are currently not supported", mp.Driver)
			}
			fallthrough
		case mount.TypeBind:
			dc.dirRewrite = &baseDirRewrite{mp.Source, baseDir}
			return nil
		}
	}
	return ErrMissingWorkdirVolume
}

// determineContainerID tries to determine the ID of the Docker container
// texd runs in.
//
// By default, this assumes the hostname is the (truncated) container ID,
// which is usually the case, unless the hostname was reconfigured.
//
// To work around this issue, we'll read a special file (/container.id)
// which, if it exists shall contain the ID. This file can be created at
// container start time:
//
//	docker run ... --cidfile=/container.id digineode:texd
//
// There's currently no way to configure the path of that file.
func determineContainerID() (string, error) {
	if cid, err := afero.ReadFile(dockerFs, "/container.id"); err == nil {
		return string(bytes.TrimSpace(cid)), nil
	}
	return hostname()
}
