package refstore

import (
	"crypto/rand"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func randomIdentifier() Identifier {
	b := make([]byte, sha256.Size)
	if _, err := rand.Read(b); err != nil {
		panic("broken math/rand.Read")
	}
	id, err := ToIdentifier(b)
	if err != nil {
		panic(err)
	}
	return id
}

func TestAccessList(t *testing.T) {
	t.Parallel()

	assert, require := assert.New(t), require.New(t)

	subject, err := NewAccessList(3, 1000)
	require.NoError(err)

	newFile := func(sz int) *FileRef {
		return &FileRef{
			ID:   randomIdentifier(),
			Size: sz,
		}
	}

	assertItems := func(fs ...*FileRef) {
		t.Helper()
		assert.Equal(len(fs), subject.items.Len())

		it := make([]*FileRef, 0, len(fs))
		for i, e := 0, subject.items.Front(); i < len(fs); i, e = i+1, e.Next() {
			it = append(it, e.Value)
		}
		assert.EqualValues(fs, it)
	}

	sm := []*FileRef{newFile(1), newFile(2), newFile(3)}
	md := newFile(500)
	lg := []*FileRef{newFile(1000), newFile(2000)}

	{ // adding two files should not evict anything
		require.Len(subject.Add(sm[0]), 0)
		assertItems(sm[0])

		require.Len(subject.Add(sm[1]), 0)
		assertItems(sm[1], sm[0])

		// peeking does not change order
		peek := subject.Peek(sm[0].ID)
		assert.Equal(sm[0].ID, peek.ID)
		assertItems(sm[1], sm[0])

		require.Len(subject.Add(sm[0]), 0) // move sm[0] to front
		assertItems(sm[0], sm[1])

		require.Len(subject.Add(sm[2]), 0)
		assertItems(sm[2], sm[0], sm[1])

		// touching moves items
		subject.Touch(sm[0].ID)
		assertItems(sm[0], sm[2], sm[1])
	}

	{ // adding another file shall evict one file
		evicted := subject.Add(md)
		require.Len(evicted, 1)
		assert.Equal(sm[1], evicted[0])
		assertItems(md, sm[0], sm[2])
	}

	{ // adding a large file evicts all smaller files
		evicted := subject.Add(lg[0])
		require.Len(evicted, 3)
		assert.EqualValues([]*FileRef{sm[2], sm[0], md}, evicted) // reverse order
		assertItems(lg[0])
	}

	{ // adding a small file evicts the large file
		evicted := subject.Add(sm[0])
		require.Len(evicted, 1)
		assert.Equal(lg[0], evicted[0])
		assertItems(sm[0])
	}

	{ // adding a single huge file keeps it
		evicted := subject.Add(lg[1])
		require.Len(evicted, 1)
		assert.Equal(sm[0], evicted[0])
		assertItems(lg[1])
	}

	{ // prime database
		evicted := subject.Prime(sm)
		assert.Len(evicted, 1)
		assert.EqualValues(lg[1], evicted[0])
	}
}
