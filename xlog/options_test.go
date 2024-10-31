package xlog

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

func TestOptionSuite(t *testing.T) {
	suite.Run(t, new(OptionSuite))
}

type OptionSuite struct {
	suite.Suite
	options
}

func (o *OptionSuite) SetupTest() {
	o.level = slog.LevelDebug
}

func (o *OptionSuite) TestLeveled() {
	err := Leveled(slog.LevelError)(&o.options)
	o.Require().NoError(err)
	o.Assert().Equal(slog.LevelError, o.level)

	err = Leveled(slog.Level(999))(&o.options)
	o.Require().NoError(err)
	o.Assert().Equal(slog.Level(999), o.level)
}

func (o *OptionSuite) TestLeveledString_valid() {
	for i, tt := range []struct {
		input    string
		expected slog.Level
	}{
		{"dbg", slog.LevelDebug},
		{"debug", slog.LevelDebug},

		{"", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},

		{"WARN", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},

		{"ERR", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"FaTaL", slog.LevelError},
	} {
		o.level = slog.Level(100 + i)
		err := LeveledString(tt.input)(&o.options)
		o.Require().NoError(err)
		o.Assert().Equal(tt.expected, o.level)
	}
}

func (o *OptionSuite) TestLeveledString_invalid() {
	o.level = slog.Level(100)
	err := LeveledString("ifno")(&o.options)
	o.Require().Equal(slog.Level(100), o.level)
	o.Assert().EqualError(err, `unknown log level: "ifno"`)
}

func (o *OptionSuite) TestWriteTo() {
	var buf bytes.Buffer
	err := WriteTo(&buf)(&o.options)
	o.Require().NoError(err)
	o.Assert().Equal(&buf, o.output)

	err = WriteTo(nil)(&o.options)
	o.Assert().EqualError(err, "invalid writer: nil")
}

func (o *OptionSuite) TestMockClock() {
	t := time.Now()
	err := MockClock(t)(&o.options)
	o.Require().NoError(err)
	o.Require().NotNil(o.clock)
	o.Assert().EqualValues(t, o.clock.Now())
}

func (o *OptionSuite) TestWithSource() {
	err := WithSource()(&o.options)
	o.Require().NoError(err)
	o.Assert().True(o.source)
}

func (o *OptionSuite) TestColor() {
	err := Color()(&o.options)
	o.Require().NoError(err)
	o.Assert().True(o.color)
}

func (o *OptionSuite) testBuildHandler(expectedLog string, opts ...Option) {
	o.T().Helper()

	var buf bytes.Buffer
	o.output = &buf

	for _, opt := range opts {
		o.Require().NoError(opt(&o.options))
	}
	o.Require().NotNil(o.buildHandler)
	h := o.buildHandler(&o.options)
	o.Require().NotNil(h)

	o.Assert().False(h.Enabled(context.Background(), -999))

	pc, _, _, ok := runtime.Caller(1)
	o.Require().True(ok)
	err := h.Handle(context.Background(), slog.NewRecord(
		time.Time{}, slog.LevelInfo, "test", pc,
	))
	o.Assert().NoError(err)
	o.Assert().EqualValues(expectedLog, buf.String())
}

func (o *OptionSuite) TestAsJSON() {
	o.testBuildHandler(`{"level":"INFO","msg":"test"}`+"\n", AsJSON())
}

func (o *OptionSuite) TestAsText() {
	o.testBuildHandler("INFO test\n", AsText())

	_, _, line, ok := runtime.Caller(0)
	o.Require().True(ok)
	msg := fmt.Sprintf("INFO options_test.go:%d test\n", line+3)
	o.testBuildHandler(msg, WithSource(), AsText())
}

func (o *OptionSuite) TestDiscard() {
	o.testBuildHandler("", Discard())
}
