package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/digineo/texd/service/middleware"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/moby/term"
	"go.uber.org/zap"
)

// newClient is swapped in tests
var newClient = func() (client.APIClient, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

// DockerClient wraps a Docker client instance and provides methods to
// pull images and start containers.
type DockerClient struct {
	log    *zap.Logger
	cli    client.APIClient
	images []types.ImageSummary
}

// NewDockerClient creates a new DockerClient. To configure the client,
// use environment variables: DOCKER_HOST, DOCKER_API_VERSION,
// DOCKER_CERT_PATH and DOCKER_TLS_VERIFY are supported.
func NewDockerClient(log *zap.Logger) (h *DockerClient, err error) {
	cli, err := newClient()
	if err != nil {
		return nil, err
	}

	if log == nil {
		log = zap.NewNop()
	}
	return &DockerClient{log, cli, nil}, nil
}

// SetImages ensures that the given image tags are present on the
// Docker host (missing images are pulled automatically). Existing
// images are not updated, unless alwaysPull is true.
//
// If stdout is a terminal, download progress is reported.
//
// SetImages also sets the DockerClients allow list from which
// containers are started.
func (dc *DockerClient) SetImages(ctx context.Context, alwaysPull bool, tags ...string) ([]string, error) {
	// A given tag may have aliases, we want to remember and allow all of them.
	knownImages := make([]types.ImageSummary, 0, len(tags))

	// collect images we need to pull
	toPull := make([]string, 0, len(tags))

	for _, tag := range tags {
		img, err := dc.findImage(ctx, tag)
		if err != nil {
			return nil, err
		}
		if img.ID == "" || alwaysPull {
			toPull = append(toPull, tag)
		} else {
			dc.log.Info("image already present", zap.String("image", tag))
			knownImages = append(knownImages, img)
		}
	}

	p := newProgressReporter(os.Stdout)
	for _, tag := range toPull {
		dc.log.Info("pulling missing image", zap.String("image", tag))
		if err := dc.pull(ctx, tag, p); err != nil {
			return nil, err
		}
		img, err := dc.findImage(ctx, tag)
		if err != nil {
			return nil, fmt.Errorf("lost previously pulled image: %v", err)
		}
		knownImages = append(knownImages, img)
	}

	// remember image tags in allow list
	dc.images = knownImages
	found := make([]string, 0, len(knownImages))
	for _, img := range knownImages {
		found = append(found, img.RepoTags...)
	}

	return found, nil
}

// have reports whether the given tag is present on the current Docker
// host.
func (dc *DockerClient) findImage(ctx context.Context, tag string) (image types.ImageSummary, err error) {
	images, err := dc.cli.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return
	}

	for _, img := range images {
		for _, t := range img.RepoTags {
			if t == tag {
				image = img
				return
			}
		}
	}
	return
}

type progess struct {
	w    io.Writer
	fd   uintptr
	term bool
}

func newProgressReporter(out *os.File) *progess {
	fd := out.Fd()
	return &progess{
		w:    out,
		fd:   fd,
		term: term.IsTerminal(fd),
	}
}

func (p *progess) report(r io.Reader) error {
	return jsonmessage.DisplayJSONMessagesStream(r, p.w, p.fd, p.term, nil)
}

// pull pulls the given image tag. Progress is reported to p, unless
// p is nil.
func (dc *DockerClient) pull(ctx context.Context, tag string, p *progess) error {
	r, err := dc.cli.ImagePull(ctx, tag, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer r.Close()

	_ = p.report(r)
	return err
}

func (dc *DockerClient) findAllowedImageID(tag string) string {
	if tag == "" && len(dc.images) > 0 {
		return dc.images[0].ID
	}
	for _, img := range dc.images {
		for _, t := range img.RepoTags {
			if t == tag {
				return img.ID
			}
		}
	}
	return ""
}

// containerWd is the work dir inside a (new) container
const containerWd = "/texd"

func (dc *DockerClient) prepareContainer(ctx context.Context, tag, wd string, cmd []string) (string, error) {
	id := dc.findAllowedImageID(tag)
	if id == "" {
		return "", fmt.Errorf("image %q not allowed", tag)
	}

	containerCfg := &container.Config{
		Image:           id,
		Cmd:             cmd,
		WorkingDir:      containerWd,
		NetworkDisabled: true,
	}

	hostCfg := &container.HostConfig{
		AutoRemove:     true,
		NetworkMode:    "none",
		ReadonlyRootfs: true,
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
	dc.log.Debug("container is ready",
		middleware.RequestIDField(ctx),
		zap.String("id", worker.ID),
		zap.String("work-dir", wd))
	return worker.ID, nil
}

func (dc *DockerClient) waitForContainer(ctx context.Context, id string) (status int64, err error) {
	statusCh, errCh := dc.cli.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return 0, err
	case s := <-statusCh:
		return s.StatusCode, nil
	}
}

// Run creates a new Docker container from the given image tag, mounts the
// working directory into it, and executes the given command in it.
func (dc *DockerClient) Run(ctx context.Context, tag, wd string, cmd []string) (string, error) {
	id, err := dc.prepareContainer(ctx, tag, wd, cmd)
	if err != nil {
		return "", err
	}

	var (
		buf      bytes.Buffer
		logErr   error
		logsDone = make(chan struct{})
	)
	go func() {
		defer close(logsDone)
		out, err := dc.cli.ContainerLogs(ctx, id, types.ContainerLogsOptions{
			ShowStderr: true,
		})
		if err != nil {
			logErr = fmt.Errorf("unable to retrieve logs: %w", err)
			return
		}
		if _, err = stdcopy.StdCopy(os.Stderr, &buf, out); err != nil {
			logErr = fmt.Errorf("unable to read logs: %w", err)
			return
		}
	}()

	if err = dc.cli.ContainerStart(ctx, id, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	status, err := dc.waitForContainer(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to run container: %w", err)
	}
	if status != 0 {
		err = fmt.Errorf("container exited with status %d", status)
	}

	<-logsDone
	if err != nil {
		return buf.String(), err
	}
	return buf.String(), logErr
}
