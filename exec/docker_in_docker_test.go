package exec

import (
	"testing"

	"github.com/docker/docker/api/types/mount"
	"github.com/stretchr/testify/suite"
)

type baseDirRewriteSuite struct {
	suite.Suite
}

func TestBaseDirRewrite(t *testing.T) {
	suite.Run(t, new(baseDirRewriteSuite))
}

// context: not running Docker-in-Docker
func (s *baseDirRewriteSuite) TestMountConfig_blank() {
	var subject *baseDirRewrite
	s.Require().Nil(subject)
	s.Assert().EqualValues(mount.Mount{
		Type:   mount.TypeBind,
		Source: "/job-blank",
		Target: containerWd,
	}, subject.MountConfig("/job-blank"))
}

// context: docker run -v /srv/texd/work:/texd ...
func (s *baseDirRewriteSuite) TestMountConfig_bindmount() {
	subject := baseDirRewrite{
		src: "/srv/texd/work",
		dst: "/texd",
	}
	s.Assert().EqualValues(mount.Mount{
		Type:   mount.TypeBind,
		Source: "/srv/texd/work/job-bind",
		Target: containerWd,
	}, subject.MountConfig("/job-bind"))
}

// context: docker run -v texd-work:/texd ...
func (s *baseDirRewriteSuite) TestMountConfig_volume() {
	subject := baseDirRewrite{
		src: "/var/lib/docker/volumes/texd-work/_data",
		dst: "/texd",
	}
	s.Assert().EqualValues(mount.Mount{
		Type:   mount.TypeBind,
		Source: "/var/lib/docker/volumes/texd-work/_data/job-volume",
		Target: containerWd,
	}, subject.MountConfig("/job-volume"))
}
