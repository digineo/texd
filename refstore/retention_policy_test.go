package refstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeepForever(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	subject := &KeepForever{}

	assert.Len(subject.Prime([]*FileRef{nil, nil}), 0)
	assert.Nil(subject.Peek(""))
	assert.Len(subject.Add(nil), 0)
}

func TestKeepPurgeOnStart(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	subject := &PurgeOnStart{}

	a, b := &FileRef{"a", 0}, &FileRef{"b", 0}

	assert.EqualValues(subject.Prime([]*FileRef{a, b}), []*FileRef{a, b})
	assert.Nil(subject.Peek(""))
	assert.Len(subject.Add(a), 0) // not evicted
}
