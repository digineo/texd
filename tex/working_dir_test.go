package tex

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"
)

type jobDirSuite struct {
	suite.Suite

	cwd string
}

func TestJobDir(t *testing.T) {
	suite.Run(t, new(jobDirSuite))
}

func (s *jobDirSuite) SetupTest() {
	wd, err := os.Getwd()
	s.Require().NoError(err)
	s.cwd = wd

	baseJobDir = ""
	texFs = afero.NewMemMapFs()
}

func (s *jobDirSuite) TearDownSuite() {
	baseJobDir = ""
	texFs = afero.NewOsFs()
}

func (s *jobDirSuite) mkdir(path string, perm fs.FileMode, uid, gid int) {
	s.T().Helper()

	s.Require().NoError(texFs.MkdirAll(path, perm))
	if uid != 0 || gid != 0 {
		s.Require().NoError(texFs.Chown(path, uid, gid))
	}
}

func (s *jobDirSuite) TestDot() {
	s.mkdir(s.cwd, 0o755, os.Geteuid(), os.Getegid())

	s.Require().NoError(SetJobBaseDir("."))
	s.Assert().Equal(s.cwd, JobBaseDir())
}

func (s *jobDirSuite) TestRelative() {
	const path = "../testdata"
	abs := filepath.Join(s.cwd, "../testdata")
	s.mkdir(s.cwd, 0o755, os.Geteuid(), os.Getegid())
	s.mkdir(abs, 0o755, os.Geteuid(), os.Getegid())

	s.Require().NoError(SetJobBaseDir(path))
	s.Assert().Equal(abs, JobBaseDir())
}

func (s *jobDirSuite) TestTemp() {
	const path = "/tmp"
	s.mkdir(path, 0o1777, 0, 0)

	s.Require().NoError(SetJobBaseDir(path))
	s.Assert().Equal(path, JobBaseDir())
}

func (s *jobDirSuite) TestBlank() {
	s.Require().NoError(SetJobBaseDir(""))
	s.Assert().Equal(os.TempDir(), JobBaseDir())
	s.Assert().Equal("", baseJobDir)
}

func (s *jobDirSuite) TestNonExistent() {
	s.Require().ErrorIs(SetJobBaseDir("/no/such/directory"), os.ErrNotExist)
	s.Assert().Equal(os.TempDir(), JobBaseDir())
	s.Assert().Equal("", baseJobDir)
}

func (s *jobDirSuite) TestFile() {
	const path = "/dir"
	s.Require().NoError(afero.WriteFile(texFs, path, nil, 0o644))

	s.Require().EqualError(SetJobBaseDir(path), `invalid working directory "/dir": invalid argument`)
}

func (s *jobDirSuite) TestNotWritable() {
	const path = "/dir"
	s.mkdir(path, 0o444, os.Geteuid(), os.Getegid())

	s.Require().ErrorIs(SetJobBaseDir(path), os.ErrPermission)
}
