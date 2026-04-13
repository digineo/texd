package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path"
	"strings"
	"syscall"
	"testing"

	"github.com/digineo/texd/xlog"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// bg is used as default context given to the apiMock stubs.
var bg = context.Background()

type mockImagePullResponse struct {
	io.ReadCloser
}

func (m *mockImagePullResponse) JSONMessages(ctx context.Context) iter.Seq2[jsonstream.Message, error] {
	// Return a no-op iterator
	return func(yield func(jsonstream.Message, error) bool) {}
}

func (m *mockImagePullResponse) Wait(ctx context.Context) error {
	return nil
}

type mockContainerLogsResult struct {
	io.ReadCloser
}

type apiMock struct {
	mock.Mock

	client.APIClient
}

func (m *apiMock) ImageList(
	ctx context.Context,
	options client.ImageListOptions,
) (client.ImageListResult, error) {
	args := m.Called(ctx, options)
	// channel trickery to allow TestSetImages create different return values
	// (and work around a limitation of the mock framework)
	return <-args.Get(0).(chan client.ImageListResult), <-args.Get(1).(chan error) //nolint:forcetypeassert
}

func (m *apiMock) ContainerInspect(
	ctx context.Context,
	id string,
	options client.ContainerInspectOptions,
) (client.ContainerInspectResult, error) {
	args := m.Called(ctx, id, options)
	return args.Get(0).(client.ContainerInspectResult), args.Error(1) //nolint:forcetypeassert
}

func (m *apiMock) ImagePull(
	ctx context.Context,
	refStr string,
	options client.ImagePullOptions,
) (client.ImagePullResponse, error) {
	args := m.Called(ctx, refStr, options)
	return args.Get(0).(client.ImagePullResponse), args.Error(1) //nolint:forcetypeassert
}

func (m *apiMock) ContainerLogs(
	ctx context.Context,
	container string,
	options client.ContainerLogsOptions,
) (client.ContainerLogsResult, error) {
	args := m.Called(ctx, container, options)
	return args.Get(0).(client.ContainerLogsResult), args.Error(1) //nolint:forcetypeassert
}

func (m *apiMock) ContainerCreate(
	ctx context.Context,
	options client.ContainerCreateOptions,
) (client.ContainerCreateResult, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(client.ContainerCreateResult), args.Error(1) //nolint:forcetypeassert
}

func (m *apiMock) ContainerStart(
	ctx context.Context,
	container string,
	options client.ContainerStartOptions,
) (client.ContainerStartResult, error) {
	args := m.Called(ctx, container, options)
	return args.Get(0).(client.ContainerStartResult), args.Error(1) //nolint:forcetypeassert
}

func (m *apiMock) ContainerWait(
	ctx context.Context,
	containerID string,
	options client.ContainerWaitOptions,
) client.ContainerWaitResult {
	args := m.Called(ctx, containerID, options)
	return args.Get(0).(client.ContainerWaitResult) //nolint:forcetypeassert
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
		log: xlog.NewDiscard(),
	}
}

func (s *dockerClientSuite) TestNewDockerClient() {
	// swap client factory
	oldClient := newClient
	newClient = func() (client.APIClient, error) {
		return s.cli, nil
	}
	defer func() { newClient = oldClient }()

	// short-circuit configureDinD()
	defer s.prepareFs(false, "")()

	cli, err := NewDockerClient(nil, "")
	s.Require().NoError(err)
	s.Require().NotNil(cli)
	s.Assert().Equal(s.cli, cli.cli)
}

func (s *dockerClientSuite) TestFindAllowedImageID() {
	s.subject.images = []image.Summary{
		{ID: "a", RepoTags: []string{"localhost/texd/minimal:v1", "localhost/texd/minimal:latest"}},
		{ID: "b", RepoTags: []string{"registry.gitlab.com/islandoftex/images/texlive:latest"}},
	}

	s.Assert().Equal("a", s.subject.findAllowedImageID("localhost/texd/minimal:latest"))
	s.Assert().Equal("b", s.subject.findAllowedImageID("registry.gitlab.com/islandoftex/images/texlive:latest"))
	s.Assert().Equal("", s.subject.findAllowedImageID("unknown"))
}

func (s *dockerClientSuite) TestFindAllowedImageID_empty() {
	s.Assert().Equal("", s.subject.findAllowedImageID("registry.gitlab.com/islandoftex/images/texlive:latest"))
}

func (s *dockerClientSuite) TestFindAllowedImageID_default() {
	s.Assert().Equal("", s.subject.findAllowedImageID(""))

	s.subject.images = []image.Summary{
		{ID: "a", RepoTags: []string{"texd/default"}},
		{ID: "b", RepoTags: []string{"texd/alternative"}},
	}
	s.Assert().Equal("a", s.subject.findAllowedImageID(""))
}

func (s *dockerClientSuite) TestFindImage() {
	const tag = "localhost/test/image:latest"

	localImages := []image.Summary{
		{ID: "does not match", RepoTags: []string{"localhost/test/image:v0.5"}},
		{ID: "matches", RepoTags: []string{tag, "localhost/test/image:v1.0"}},
		{ID: "matches not", RepoTags: []string{"localhost/test/image:v0.9"}},
	}
	imgCh := make(chan client.ImageListResult, 1)
	imgCh <- client.ImageListResult{Items: localImages}
	close(imgCh)

	errCh := make(chan error)
	close(errCh)

	s.cli.On("ImageList", bg, client.ImageListOptions{}).Return(imgCh, errCh)

	img, err := s.subject.findImage(bg, tag)
	s.Require().NoError(err)
	s.Assert().Equal("matches", img.ID)
}

func (s *dockerClientSuite) TestFindImage_failure() {
	imgCh := make(chan client.ImageListResult, 1)
	imgCh <- client.ImageListResult{Items: []image.Summary{}}
	close(imgCh)

	errCh := make(chan error, 1)
	errCh <- errors.New("test-list-error")
	close(errCh)

	s.cli.On("ImageList", bg, client.ImageListOptions{}).Return(imgCh, errCh)

	_, err := s.subject.findImage(bg, "test:latest")
	s.Require().EqualError(err, "test-list-error")
}

func (s *dockerClientSuite) prepareFs(inContainer bool, cidFileContent string) func() {
	fs := afero.NewMemMapFs()
	if inContainer {
		err := afero.WriteFile(fs, "/.dockerenv", nil, 0o644)
		s.Require().NoError(err)

		if cidFileContent != "" {
			err = afero.WriteFile(fs, "/container.id", []byte(cidFileContent), 0o644)
			s.Require().NoError(err)
		}
	}

	dockerFs = fs
	return func() {
		dockerFs = afero.NewOsFs()
	}
}

func parseMount(vol string) (m container.MountPoint) {
	parts := strings.SplitN(vol, ":", 3)
	if len(parts) != 2 {
		panic("unsupported")
	}
	m.Source = parts[0]
	m.Destination = parts[1]
	if path.IsAbs(parts[0]) {
		m.Type = mount.TypeBind
	} else {
		m.Type = mount.TypeVolume
		m.Source = fmt.Sprintf("/var/lib/docker/volumes/%s/_data", parts[0])
		m.Driver = "local"
	}
	return
}

func (s *dockerClientSuite) TestConfigureDinD_outsideContainer() {
	defer s.prepareFs(false, "")()

	s.Require().NoError(s.subject.configureDinD("/texd"))
	s.Assert().Nil(s.subject.dirRewrite)
}

func (s *dockerClientSuite) TestConfigureDinD_blankBaseDir() {
	defer s.prepareFs(true, "")()

	s.Require().NoError(s.subject.configureDinD(""))
	s.Assert().Nil(s.subject.dirRewrite)
}

type failFs struct {
	afero.Fs
	fails map[string]error
}

func (fs *failFs) Open(filename string) (afero.File, error) {
	if err := fs.fails[filename]; err != nil {
		return nil, err
	}
	return fs.Fs.Open(filename)
}

func (s *dockerClientSuite) TestConfigureDinD_unreadableCIDFile() {
	hostname = func() (string, error) { return "", syscall.EFAULT }
	defer func() { hostname = os.Hostname }()

	defer s.prepareFs(true, "id")()
	dockerFs = &failFs{dockerFs, map[string]error{
		"/container.id": io.ErrUnexpectedEOF,
	}}

	s.Require().EqualError(s.subject.configureDinD("/"),
		"cannot determine texd container ID: bad address")
	s.Assert().Nil(s.subject.dirRewrite)
}

func (s *dockerClientSuite) TestConfigureDinD_invalidCID() {
	defer s.prepareFs(true, "id")()

	s.cli.On("ContainerInspect", bg, "id", client.ContainerInspectOptions{}).Return(
		client.ContainerInspectResult{},
		errors.New("Cannot connect to the Docker daemon at localhost. Is the docker daemon running?"))

	s.Require().EqualError(s.subject.configureDinD("/"),
		"cannot determine texd container: Cannot connect to the Docker daemon at localhost. Is the docker daemon running?")
	s.Assert().Nil(s.subject.dirRewrite)
}

func (s *dockerClientSuite) TestConfigureDinD_missingWorkdirVolume() {
	defer s.prepareFs(true, "id")()

	s.cli.On("ContainerInspect", bg, "id", client.ContainerInspectOptions{}).Return(
		client.ContainerInspectResult{}, nil)

	s.Require().EqualError(s.subject.configureDinD("/texd"),
		"missing Docker volume or bind mount for work directory")
	s.Assert().Nil(s.subject.dirRewrite)
}

func (s *dockerClientSuite) TestConfigureDinD_withBindMount() {
	defer s.prepareFs(true, "our-id")()

	s.cli.On("ContainerInspect", bg, "our-id", client.ContainerInspectOptions{}).Return(
		client.ContainerInspectResult{
			Container: container.InspectResponse{
				Mounts: []container.MountPoint{
					parseMount("/var/run/docker.sock:/var/run/docker.sock"),
					parseMount("/srv/texd/work:/texd"),
				},
			},
		},
		nil)

	s.Assert().Nil(s.subject.dirRewrite)
	s.Require().NoError(s.subject.configureDinD("/texd"))
	s.Assert().EqualValues(&baseDirRewrite{
		src: "/srv/texd/work",
		dst: "/texd",
	}, s.subject.dirRewrite)
}

func (s *dockerClientSuite) TestConfigureDinD_nonLocalDriver() {
	defer s.prepareFs(true, "id")()

	s.cli.On("ContainerInspect", bg, "id", client.ContainerInspectOptions{}).Return(
		client.ContainerInspectResult{
			Container: container.InspectResponse{
				Mounts: []container.MountPoint{{
					Type:        mount.TypeVolume,
					Driver:      "div/0",
					Destination: "/texd",
				}},
			},
		},
		nil)

	s.Require().EqualError(s.subject.configureDinD("/texd"),
		"div/0 volume binds are currently not supported")
	s.Assert().Nil(s.subject.dirRewrite)
}

func (s *dockerClientSuite) TestConfigureDinD_withVolume() {
	defer s.prepareFs(true, "our-id")()

	s.cli.On("ContainerInspect", bg, "our-id", client.ContainerInspectOptions{}).Return(
		client.ContainerInspectResult{
			Container: container.InspectResponse{
				Mounts: []container.MountPoint{
					parseMount("/var/run/docker.sock:/var/run/docker.sock"),
					parseMount("texd-work:/texd"),
				},
			},
		},
		nil)

	s.Assert().Nil(s.subject.dirRewrite)
	s.Require().NoError(s.subject.configureDinD("/texd"))
	s.Assert().EqualValues(&baseDirRewrite{
		src: "/var/lib/docker/volumes/texd-work/_data",
		dst: "/texd",
	}, s.subject.dirRewrite)
}

func (s *dockerClientSuite) TestConfigureDinD_withoutCIDFile() {
	defer s.prepareFs(true, "")()

	hostname, err := os.Hostname()
	s.Require().NoError(err)

	s.cli.On("ContainerInspect", bg, hostname, client.ContainerInspectOptions{}).Return(
		client.ContainerInspectResult{
			Container: container.InspectResponse{
				Mounts: []container.MountPoint{
					parseMount("/var/run/docker.sock:/var/run/docker.sock"),
					parseMount("/srv/texd/work:/texd"),
				},
			},
		},
		nil)

	s.Assert().Nil(s.subject.dirRewrite)
	s.Require().NoError(s.subject.configureDinD("/texd"))
	s.Assert().EqualValues(&baseDirRewrite{
		src: "/srv/texd/work",
		dst: "/texd",
	}, s.subject.dirRewrite)
}

func (s *dockerClientSuite) TestPull() {
	var buf bytes.Buffer

	mockResponse := &mockImagePullResponse{ReadCloser: io.NopCloser(&buf)}
	s.cli.On("ImagePull", bg, "localhost/test/image", client.ImagePullOptions{}).
		Return(mockResponse, nil)

	p := newProgressReporter(os.Stderr)

	s.Require().NoError(s.subject.pull(bg, "localhost/test/image", p))
}

func (s *dockerClientSuite) TestPull_failure() {
	mockResponse := &mockImagePullResponse{ReadCloser: io.NopCloser(nil)}
	s.cli.On("ImagePull", bg, "test:latest", client.ImagePullOptions{}).
		Return(mockResponse, errors.New("test-pull-failure"))

	err := s.subject.pull(bg, "test:latest", nil)
	s.Require().EqualError(err, "test-pull-failure")
}

func (s *dockerClientSuite) TestSetImages() {
	localImages := []image.Summary{
		{ID: "a", RepoTags: []string{"test:v1"}},
		{ID: "b", RepoTags: []string{"test:v3"}},
		{ID: "c", RepoTags: []string{"test:v2"}},
	}

	// ImageList is called three times
	imgCh := make(chan client.ImageListResult, 3)
	imgCh <- client.ImageListResult{Items: localImages}      // find(test:v3) → ok
	imgCh <- client.ImageListResult{}                        // find(test:v4) → not found → pull
	imgCh <- client.ImageListResult{Items: []image.Summary{{ // find(test:v4) → ok
		ID:       "d",
		RepoTags: []string{"test:v4", "test:latest"},
	}}}
	close(imgCh)

	errCh := make(chan error)
	close(errCh)

	s.cli.On("ImageList", bg, client.ImageListOptions{}).Return(imgCh, errCh)
	mockResponse := &mockImagePullResponse{ReadCloser: io.NopCloser(&bytes.Buffer{})}
	s.cli.On("ImagePull", bg, "test:v4", client.ImagePullOptions{}).
		Return(mockResponse, nil)

	found, err := s.subject.SetImages(bg, false, "test:v3", "test:v4")
	s.Require().NoError(err)
	s.Assert().ElementsMatch([]string{"test:v3", "test:v4", "test:latest"}, found)
}

func (s *dockerClientSuite) TestSetImages_errFindImage() {
	imgCh := make(chan client.ImageListResult)
	close(imgCh)

	errCh := make(chan error, 1)
	errCh <- errors.New("unable to resolve")
	close(errCh)

	s.cli.On("ImageList", bg, client.ImageListOptions{}).
		Return(imgCh, errCh)

	found, err := s.subject.SetImages(bg, false, "test:v0")
	s.Require().EqualError(err, "unable to resolve")
	s.Assert().Empty(found)
}

func (s *dockerClientSuite) TestSetImages_errPullImage() {
	imgCh := make(chan client.ImageListResult, 1)
	imgCh <- client.ImageListResult{} // find(test:v0) → not found → pull
	close(imgCh)

	errCh := make(chan error)
	close(errCh)

	s.cli.On("ImageList", bg, client.ImageListOptions{}).Return(imgCh, errCh)
	mockResponse := &mockImagePullResponse{ReadCloser: io.NopCloser(&bytes.Buffer{})}
	s.cli.On("ImagePull", bg, "test:v0", client.ImagePullOptions{}).
		Return(mockResponse, errors.New("connection reset"))

	found, err := s.subject.SetImages(bg, false, "test:v0")
	s.Require().EqualError(err, "connection reset")
	s.Assert().Empty(found)
}

func (s *dockerClientSuite) TestSetImages_errLosingImageA() {
	imgCh := make(chan client.ImageListResult)
	close(imgCh)

	errCh := make(chan error, 2)
	errCh <- nil
	errCh <- errors.New("image-list-err")
	close(errCh)

	s.cli.On("ImageList", bg, client.ImageListOptions{}).Return(imgCh, errCh)
	mockResponse := &mockImagePullResponse{ReadCloser: io.NopCloser(&bytes.Buffer{})}
	s.cli.On("ImagePull", bg, "test:v0", client.ImagePullOptions{}).
		Return(mockResponse, nil)

	found, err := s.subject.SetImages(bg, true, "test:v0")
	s.Require().EqualError(err, "lost previously pulled image: image-list-err")
	s.Assert().Empty(found)
}

func (s *dockerClientSuite) TestSetImages_errLosingImageB() {
	imgCh := make(chan client.ImageListResult)
	close(imgCh)

	errCh := make(chan error)
	close(errCh)

	s.cli.On("ImageList", bg, client.ImageListOptions{}).Return(imgCh, errCh)
	mockResponse := &mockImagePullResponse{ReadCloser: io.NopCloser(&bytes.Buffer{})}
	s.cli.On("ImagePull", bg, "test:v0", client.ImagePullOptions{}).
		Return(mockResponse, nil)

	found, err := s.subject.SetImages(bg, true, "test:v0")
	s.Require().EqualError(err, "lost previously pulled image (test:v0)")
	s.Assert().Empty(found)
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
		s.subject.images = append(s.subject.images, image.Summary{
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

	s.cli.On("ContainerCreate", bg, client.ContainerCreateOptions{
		Config:     ccfg,
		HostConfig: hcfg,
	}).Return(client.ContainerCreateResult{ID: runningID}, startErr)
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
	mockLogs := &mockContainerLogsResult{ReadCloser: io.NopCloser(&logs)}
	s.cli.On("ContainerLogs", bg, runningID, client.ContainerLogsOptions{
		ShowStderr: true,
	}).Return(mockLogs, nil)

	s.cli.On("ContainerStart", bg, runningID, client.ContainerStartOptions{}).
		Return(client.ContainerStartResult{}, nil)

	statusCh := make(chan container.WaitResponse, 1)
	statusCh <- container.WaitResponse{StatusCode: 0}
	errCh := make(chan error, 1)
	s.cli.On("ContainerWait", bg, runningID, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning}).
		Return(client.ContainerWaitResult{Result: statusCh, Error: errCh})

	out, err := s.subject.Run(bg, "texd", "/job", []string{"latexmk"})
	s.Require().NoError(err)
	s.Assert().Empty(out) // simulating logs is hard, ignore for now
}

func (s *dockerClientSuite) TestRun_errContainerCreate() {
	const runningID = "deadbeef"
	s.mockContainerCreate("texd", "/job", []string{"latexmk"},
		runningID, io.ErrClosedPipe)

	out, err := s.subject.Run(bg, "texd", "/job", []string{"latexmk"})
	s.Require().EqualError(err, "failed to create container: io: read/write on closed pipe")
	s.Assert().Equal("", out)
}

func (s *dockerClientSuite) TestRun_errRetrieveLogs() {
	const runningID = "7ea"
	s.mockContainerCreate("texd", "/job", []string{"latexmk"},
		runningID, nil)

	mockLogs := &mockContainerLogsResult{ReadCloser: io.NopCloser(nil)}
	s.cli.On("ContainerLogs", bg, runningID, client.ContainerLogsOptions{
		ShowStderr: true,
	}).Return(mockLogs, errors.New("failed"))

	s.cli.On("ContainerStart", bg, runningID, client.ContainerStartOptions{}).
		Return(client.ContainerStartResult{}, nil)

	statusCh := make(chan container.WaitResponse, 1)
	statusCh <- container.WaitResponse{StatusCode: 0}
	errCh := make(chan error)
	s.cli.On("ContainerWait", bg, runningID, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning}).
		Return(client.ContainerWaitResult{Result: statusCh, Error: errCh})

	out, err := s.subject.Run(bg, "texd", "/job", []string{"latexmk"})
	s.Require().EqualError(err, "unable to retrieve logs: failed")
	s.Assert().Equal("", out)
}

type failReader struct{ error }

func (f *failReader) Read([]byte) (n int, err error) {
	return 0, f.error
}

func (s *dockerClientSuite) TestRun_errReadLogs() {
	const runningID = "7ea"
	s.mockContainerCreate("texd", "/job", []string{"latexmk"},
		runningID, nil)

	mockLogs := &mockContainerLogsResult{ReadCloser: io.NopCloser(&failReader{errors.New("copy failure")})}
	s.cli.On("ContainerLogs", bg, runningID, client.ContainerLogsOptions{
		ShowStderr: true,
	}).Return(mockLogs, nil)

	s.cli.On("ContainerStart", bg, runningID, client.ContainerStartOptions{}).
		Return(client.ContainerStartResult{}, nil)

	statusCh := make(chan container.WaitResponse, 1)
	statusCh <- container.WaitResponse{StatusCode: 0}
	errCh := make(chan error)
	s.cli.On("ContainerWait", bg, runningID, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning}).
		Return(client.ContainerWaitResult{Result: statusCh, Error: errCh})

	out, err := s.subject.Run(bg, "texd", "/job", []string{"latexmk"})
	s.Require().EqualError(err, "unable to read logs: copy failure")
	s.Assert().Equal("", out)
}

func (s *dockerClientSuite) TestRun_errContainerStart() {
	const runningID = "c0ffee"
	s.mockContainerCreate("texd", "/job", []string{"latexmk"},
		runningID, nil)

	mockLogs := &mockContainerLogsResult{ReadCloser: io.NopCloser(&bytes.Buffer{})}
	s.cli.On("ContainerLogs", bg, runningID, client.ContainerLogsOptions{
		ShowStderr: true,
	}).Return(mockLogs, nil)

	s.cli.On("ContainerStart", bg, runningID, client.ContainerStartOptions{}).
		Return(client.ContainerStartResult{}, errors.New("dockerd busy"))

	_, err := s.subject.Run(bg, "texd", "/job", []string{"latexmk"})
	s.Require().EqualError(err, "failed to start container: dockerd busy")
}

func (s *dockerClientSuite) TestRun_errWaitForContainer() {
	const runningID = "c0ffee"
	s.mockContainerCreate("texd", "/job", []string{"latexmk"},
		runningID, nil)

	mockLogs := &mockContainerLogsResult{ReadCloser: io.NopCloser(&bytes.Buffer{})}
	s.cli.On("ContainerLogs", bg, runningID, client.ContainerLogsOptions{
		ShowStderr: true,
	}).Return(mockLogs, nil)

	s.cli.On("ContainerStart", bg, runningID, client.ContainerStartOptions{}).
		Return(client.ContainerStartResult{}, nil)

	statusCh := make(chan container.WaitResponse)
	errCh := make(chan error, 1)
	errCh <- errors.New("unexpected restart")
	s.cli.On("ContainerWait", bg, runningID, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning}).
		Return(client.ContainerWaitResult{Result: statusCh, Error: errCh})

	_, err := s.subject.Run(bg, "texd", "/job", []string{"latexmk"})
	s.Require().EqualError(err, "failed to run container: unexpected restart")
}

func (s *dockerClientSuite) TestRun_errExitStatus() {
	const runningID = "c0ffee"
	s.mockContainerCreate("texd", "/job", []string{"latexmk"},
		runningID, nil)

	var logs bytes.Buffer
	mockLogs := &mockContainerLogsResult{ReadCloser: io.NopCloser(&logs)}
	s.cli.On("ContainerLogs", bg, runningID, client.ContainerLogsOptions{
		ShowStderr: true,
	}).Return(mockLogs, nil)

	s.cli.On("ContainerStart", bg, runningID, client.ContainerStartOptions{}).
		Return(client.ContainerStartResult{}, nil)

	statusCh := make(chan container.WaitResponse, 1)
	statusCh <- container.WaitResponse{StatusCode: 127}
	errCh := make(chan error, 1)
	s.cli.On("ContainerWait", bg, runningID, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning}).
		Return(client.ContainerWaitResult{Result: statusCh, Error: errCh})

	_, err := s.subject.Run(bg, "texd", "/job", []string{"latexmk"})
	s.Require().EqualError(err, "container exited with status 127")
}
