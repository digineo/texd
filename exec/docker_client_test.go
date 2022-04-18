package exec

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// bg is used as default context given to the apiMock stubs.
var bg = context.Background()

type apiMock struct {
	mock.Mock

	client.APIClient
}

func (m *apiMock) ImageList(
	ctx context.Context,
	options types.ImageListOptions,
) ([]types.ImageSummary, error) {
	args := m.Called(ctx, options)
	// channel trickery to allow TestSetImages create different return values
	// (and work around a limitation of the mock framework)
	return <-args.Get(0).(chan []types.ImageSummary), args.Error(1)
}

func (m *apiMock) ContainerInspect(
	ctx context.Context,
	id string,
) (types.ContainerJSON, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(types.ContainerJSON), args.Error(1)
}

func (m *apiMock) ImagePull(
	ctx context.Context,
	ref string,
	options types.ImagePullOptions,
) (io.ReadCloser, error) {
	args := m.Called(ctx, ref, options)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *apiMock) ContainerLogs(
	ctx context.Context,
	container string,
	options types.ContainerLogsOptions,
) (io.ReadCloser, error) {
	args := m.Called(ctx, container, options)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *apiMock) ContainerCreate(
	ctx context.Context,
	config *container.Config,
	host *container.HostConfig,
	networking *network.NetworkingConfig,
	platform *specs.Platform,
	containerName string,
) (container.ContainerCreateCreatedBody, error) {
	args := m.Called(ctx, config, host, networking, platform, containerName)
	return args.Get(0).(container.ContainerCreateCreatedBody), args.Error(1)
}

func (m *apiMock) ContainerStart(
	ctx context.Context,
	container string,
	options types.ContainerStartOptions,
) error {
	args := m.Called(ctx, container, options)
	return args.Error(0)
}

func (m *apiMock) ContainerWait(
	ctx context.Context,
	containerID string,
	condition container.WaitCondition,
) (<-chan container.ContainerWaitOKBody, <-chan error) {
	args := m.Called(ctx, containerID, condition)
	return args.Get(0).(chan container.ContainerWaitOKBody), args.Get(1).(chan error)
}

type dockerClientSuite struct {
	suite.Suite

	cli     *apiMock
	subject *DockerClient
}

func TestDockerClient(t *testing.T) {
	suite.Run(t, new(dockerClientSuite))
}

func (s *dockerClientSuite) SetupTest() {
	s.cli = &apiMock{}
	s.subject = &DockerClient{
		cli: s.cli,
		log: zap.NewNop(),
	}
}

func (s *dockerClientSuite) TestNewDockerClient() {
	// swap client factory
	oldClient := newClient
	newClient = func() (client.APIClient, error) {
		return s.cli, nil
	}
	defer func() { newClient = oldClient }()

	cli, err := NewDockerClient(nil)
	s.Require().NoError(err)
	s.Require().NotNil(cli)
	s.Assert().Equal(s.cli, cli.cli)
}

func (s *dockerClientSuite) TestFindAllowedImageID() {
	s.subject.images = []types.ImageSummary{
		{ID: "a", RepoTags: []string{"localhost/texd/minimal:v1", "localhost/texd/minimal:latest"}},
		{ID: "b", RepoTags: []string{"texlive/texlive:latest"}},
	}

	s.Assert().Equal("a", s.subject.findAllowedImageID("localhost/texd/minimal:latest"))
	s.Assert().Equal("b", s.subject.findAllowedImageID("texlive/texlive:latest"))
	s.Assert().Equal("", s.subject.findAllowedImageID("unknown"))
}

func (s *dockerClientSuite) TestFindAllowedImageID_empty() {
	s.Assert().Equal("", s.subject.findAllowedImageID("texlive/texlive:latest"))
}

func (s *dockerClientSuite) TestFindAllowedImageID_default() {
	s.Assert().Equal("", s.subject.findAllowedImageID(""))

	s.subject.images = []types.ImageSummary{
		{ID: "a", RepoTags: []string{"texd/default"}},
		{ID: "b", RepoTags: []string{"texd/alternative"}},
	}
	s.Assert().Equal("a", s.subject.findAllowedImageID(""))
}

func (s *dockerClientSuite) TestFindImage() {
	const tag = "localhost/test/image:latest"

	localImages := []types.ImageSummary{
		{ID: "does not match", RepoTags: []string{"localhost/test/image:v0.5"}},
		{ID: "matches", RepoTags: []string{tag, "localhost/test/image:v1.0"}},
		{ID: "matches not", RepoTags: []string{"localhost/test/image:v0.9"}},
	}
	imgCh := make(chan []types.ImageSummary, 1)
	imgCh <- localImages
	close(imgCh)

	s.cli.On("ImageList", bg, types.ImageListOptions{}).Return(imgCh, nil)

	img, err := s.subject.findImage(bg, tag)
	s.Require().NoError(err)
	s.Assert().Equal("matches", img.ID)
}

func (s *dockerClientSuite) TestFindImage_failure() {
	imgCh := make(chan []types.ImageSummary, 1)
	imgCh <- []types.ImageSummary{}
	close(imgCh)

	s.cli.On("ImageList", bg, types.ImageListOptions{}).
		Return(imgCh, errors.New("test-list-error"))

	_, err := s.subject.findImage(bg, "test:latest")
	s.Require().EqualError(err, "test-list-error")
}

func (s *dockerClientSuite) TestPull() {
	var buf bytes.Buffer

	s.cli.On("ImagePull", bg, "localhost/test/image", types.ImagePullOptions{}).
		Return(io.NopCloser(&buf), nil)

	p := newProgressReporter(os.Stderr)

	s.Require().NoError(s.subject.pull(bg, "localhost/test/image", p))
}

func (s *dockerClientSuite) TestPull_failure() {
	s.cli.On("ImagePull", bg, "test:latest", types.ImagePullOptions{}).
		Return(io.NopCloser(nil), errors.New("test-pull-failure"))

	err := s.subject.pull(bg, "test:latest", nil)
	s.Require().EqualError(err, "test-pull-failure")
}

func (s *dockerClientSuite) TestSetImages() {
	localImages := []types.ImageSummary{
		{ID: "a", RepoTags: []string{"test:v1"}},
		{ID: "b", RepoTags: []string{"test:v3"}},
		{ID: "c", RepoTags: []string{"test:v2"}},
	}

	// ImageList is called three times
	imgCh := make(chan []types.ImageSummary, 3)
	imgCh <- localImages            // find(test:v3) → ok
	imgCh <- nil                    // find(test:v4) → not found → pull
	imgCh <- []types.ImageSummary{{ // find(test:v4) → ok
		ID:       "d",
		RepoTags: []string{"test:v4", "test:latest"},
	}}
	close(imgCh)

	p := io.NopCloser(&bytes.Buffer{}) // for progress reporter
	s.cli.On("ImageList", bg, types.ImageListOptions{}).Return(imgCh, nil)
	s.cli.On("ImagePull", bg, "test:v4", types.ImagePullOptions{}).Return(p, nil)

	found, err := s.subject.SetImages(bg, false, "test:v3", "test:v4")
	s.Require().NoError(err)
	s.Assert().ElementsMatch([]string{"test:v3", "test:v4", "test:latest"}, found)
}

func (s *dockerClientSuite) mockContainerCreate(tag, wd string, cmd []string, runningID string, startErr error) {
	var haveImage bool
search:
	for _, img := range s.subject.images {
		for _, t := range img.RepoTags {
			if t == tag {
				haveImage = true
				break search
			}
		}
	}
	if !haveImage {
		s.subject.images = append(s.subject.images, types.ImageSummary{
			ID:       "test",
			RepoTags: []string{tag},
		})
	}

	ccfg := &container.Config{
		Image:           "test",
		Cmd:             cmd,
		WorkingDir:      containerWd,
		NetworkDisabled: true,
	}
	hcfg := &container.HostConfig{
		AutoRemove:     true,
		NetworkMode:    "none",
		ReadonlyRootfs: true,
		Mounts: []mount.Mount{{
			Type:   mount.TypeBind,
			Source: wd,
			Target: containerWd,
		}},
	}
	var ncfg *network.NetworkingConfig // nil!
	var pltf *specs.Platform           // nil!

	s.cli.On("ContainerCreate", bg, ccfg, hcfg, ncfg, pltf, "").
		Return(container.ContainerCreateCreatedBody{ID: runningID}, startErr)
}

func (s *dockerClientSuite) TestPrepareContainer() {
	const wd = "/texd/job-42"
	cmd := []string{"echo", "1"}

	s.mockContainerCreate("test:latest", wd, cmd,
		"worker-1", nil)

	id, err := s.subject.prepareContainer(bg, "", wd, cmd)
	s.Require().NoError(err)
	s.Assert().Equal("worker-1", id)
}

func (s *dockerClientSuite) TestPrepareContainer_unknownImage() {
	id, err := s.subject.prepareContainer(bg, "un:known", "/", nil)
	s.Require().EqualError(err, `image "un:known" not allowed`)
	s.Assert().Equal("", id)
}

func (s *dockerClientSuite) TestPrepareContainer_failToStart() {
	s.mockContainerCreate("test:latest", "/", []string{"true"},
		"", errors.New("test-failure"))

	id, err := s.subject.prepareContainer(bg, "", "/", []string{"true"})
	s.Require().EqualError(err, "failed to create container: test-failure")
	s.Assert().Equal("", id)
}

func (s *dockerClientSuite) TestRun() {
	const runningID = "c0ffee"
	s.mockContainerCreate("texd", "/job", []string{"latexmk"},
		runningID, nil)

	var logs bytes.Buffer
	s.cli.On("ContainerLogs", bg, runningID, types.ContainerLogsOptions{
		ShowStderr: true,
	}).Return(io.NopCloser(&logs), nil)

	s.cli.On("ContainerStart", bg, runningID, types.ContainerStartOptions{}).
		Return(nil)

	statusCh := make(chan container.ContainerWaitOKBody, 1)
	statusCh <- container.ContainerWaitOKBody{StatusCode: 0}
	errCh := make(chan error, 1)
	s.cli.On("ContainerWait", bg, runningID, container.WaitConditionNotRunning).
		Return(statusCh, errCh)

	out, err := s.subject.Run(bg, "texd", "/job", []string{"latexmk"})
	s.Require().NoError(err)
	s.Assert().Empty(out) // simulating logs is hard, ignore for now
}
