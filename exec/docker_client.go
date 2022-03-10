package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/moby/term"
)

// DockerClient wraps a Docker client instance and provides methods to
// pull images and start containers.
type DockerClient struct {
	cli  *client.Client
	tags []string
}

// NewDockerClient creates a new DockerClient. To configure the client,
// use environment variables: DOCKER_HOST, DOCKER_API_VERSION,
// DOCKER_CERT_PATH and DOCKER_TLS_VERIFY are supported.
func NewDockerClient() (h *DockerClient, err error) {
	h = &DockerClient{}
	h.cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	return h, err
}

// SetImages ensures that the given image tags are present on the
// Docker host (missing images are pulled automatically). Existing
// images are not updated, unless alwaysPull is true.
//
// If stdout is a terminal, download progress is reported.
//
// SetImages also sets the DockerClients allow list from which
// containers are started.
func (dc *DockerClient) SetImages(ctx context.Context, alwaysPull bool, tags ...string) error {
	// collect images we need to pull
	toPull := make([]string, 0, len(tags))
	for _, tag := range tags {
		found, err := dc.have(ctx, tag)
		if err != nil {
			return err
		}
		if found && !alwaysPull {
			log.Println("image already present:", tag)
		} else {
			toPull = append(toPull, tag)
		}
	}

	// only report progress, if terminal is a TTY
	var p *progess
	if fd := os.Stdout.Fd(); term.IsTerminal(fd) {
		p = &progess{os.Stdout, fd}
	}

	for _, tag := range toPull {
		log.Println("pulling missing image:", tag)
		if err := dc.pull(ctx, tag, p); err != nil {
			return err
		}
	}

	// remember image tags in allow list
	dc.tags = tags
	return nil
}

// have reports whether the given tag is present on the current Docker
// host.
func (dc *DockerClient) have(ctx context.Context, tag string) (bool, error) {
	images, err := dc.cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return false, fmt.Errorf("docker: failed to list images: %w", err)
	}

	for _, img := range images {
		for _, t := range img.RepoTags {
			if t == tag {
				return true, nil
			}
		}
	}

	return false, nil
}

type progess struct {
	w  io.Writer
	fd uintptr
}

// pull pulls the given image tag. Progress is reported to p, unless
// p is nil.
func (dc *DockerClient) pull(ctx context.Context, tag string, p *progess) error {
	r, err := dc.cli.ImagePull(ctx, tag, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer r.Close()

	if p == nil {
		_, err = io.Copy(io.Discard, r)
	} else {
		err = jsonmessage.DisplayJSONMessagesStream(r, p.w, p.fd, true, nil)
	}
	return err
}

func (dc *DockerClient) allowed(tag string) bool {
	for _, t := range dc.tags {
		if t == tag {
			return true
		}
	}
	return false
}

func (dc *DockerClient) Run(ctx context.Context, tag string, wd string, cmd []string) (string, error) {
	if !dc.allowed(tag) {
		return "", fmt.Errorf("image %q not allowed", tag)
	}

	const containerWd = "/texd"
	containerCfg := &container.Config{
		Image:           tag,
		Cmd:             cmd,
		WorkingDir:      containerWd,
		NetworkDisabled: true,
	}

	hostCfg := &container.HostConfig{
		AutoRemove:  true,
		NetworkMode: "none",
		Mounts: []mount.Mount{{
			Type:   mount.TypeBind,
			Source: wd,
			Target: containerWd,
		}},
	}

	worker, err := dc.cli.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}
	if err = dc.cli.ContainerStart(ctx, worker.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	status, errs := dc.cli.ContainerWait(ctx, worker.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errs:
		return "", fmt.Errorf("failed to run container: %w", err)
	case s := <-status:
		if s.Error != nil {
			return "", fmt.Errorf("failed to run container: %s", s.Error.Message)
		}
	}

	opts := types.ContainerLogsOptions{ShowStderr: true}
	out, err := dc.cli.ContainerLogs(ctx, worker.ID, opts)
	if err != nil {
		return "", fmt.Errorf("unable to retrieve logs: %w", err)
	}

	var buf bytes.Buffer
	if _, err = stdcopy.StdCopy(io.Discard, &buf, out); err != nil {
		return "", fmt.Errorf("unable to read logs: %w", err)
	}

	return buf.String(), nil
}
