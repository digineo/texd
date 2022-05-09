package tex

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorWithCategory(t *testing.T) {
	t.Parallel()

	{
		err := ErrWithCategory{message: "message"}
		assert.EqualError(t, &err, "message")

		json, marshal := err.MarshalJSON()
		require.NoError(t, marshal)
		assert.Equal(t, `{"category":"unknown","error":"message"}`, string(json))
	}
	{
		err := ErrWithCategory{cat: inputErr, message: "message"}
		assert.EqualError(t, &err, "message")

		json, marshal := err.MarshalJSON()
		require.NoError(t, marshal)
		assert.Equal(t, `{"category":"input","error":"message"}`, string(json))
	}
	{
		err := ErrWithCategory{message: "message", extra: KV{"foo": 42}}
		assert.EqualError(t, &err, "message")

		json, marshal := err.MarshalJSON()
		require.NoError(t, marshal)
		// extra data is appended
		assert.Equal(t, `{"category":"unknown","error":"message","foo":42}`, string(json))
	}
	{
		err := ErrWithCategory{message: "message", extra: KV{"foo": 42, "category": "a", "error": "b"}}
		assert.EqualError(t, &err, "message")

		json, marshal := err.MarshalJSON()
		require.NoError(t, marshal)
		// reserved keywords are omitted
		assert.Equal(t, `{"category":"unknown","error":"message","foo":42}`, string(json))
	}
	{
		err := ErrWithCategory{message: "message", cause: io.EOF}
		// include cause
		assert.EqualError(t, &err, "message: EOF")

		json, marshal := err.MarshalJSON()
		require.NoError(t, marshal)
		// omit cause
		assert.Equal(t, `{"category":"unknown","error":"message"}`, string(json))
	}
}

func TestExtendError_blank(t *testing.T) {
	t.Parallel()

	{
		err := InputError("message", nil, nil)
		ExtendError(err, nil)
		assert.Len(t, err.(*ErrWithCategory).extra, 0)
	}
	{
		err := InputError("message", nil, KV{})
		ExtendError(err, nil)
		assert.Len(t, err.(*ErrWithCategory).extra, 0)
	}
	{
		err := InputError("message", nil, nil)
		ExtendError(err, KV{})
		assert.Len(t, err.(*ErrWithCategory).extra, 0)
	}
	{
		err := InputError("message", nil, KV{})
		ExtendError(err, KV{})
		assert.Len(t, err.(*ErrWithCategory).extra, 0)
	}
}

func TestExtendError_extendsErr(t *testing.T) {
	t.Parallel()

	{
		err := InputError("message", nil, nil)
		ExtendError(err, KV{"foo": "bar"})
		assert.Equal(t, "bar", err.(*ErrWithCategory).extra["foo"])
	}
	{
		err := InputError("message", nil, KV{"foo": "bar"})
		ExtendError(err, nil)
		assert.Equal(t, "bar", err.(*ErrWithCategory).extra["foo"])
	}
	{
		err := InputError("message", nil, KV{"foo": "original"})
		ExtendError(err, KV{"foo": "bar"})
		assert.Equal(t, "bar", err.(*ErrWithCategory).extra["foo"])
	}
}

func TestExtendError_onlyCatErrors(t *testing.T) {
	t.Parallel()

	err := io.EOF
	ExtendError(err, KV{"foo": "bar"})
	assert.True(t, err == io.EOF)
}

func TestErrorIs(t *testing.T) {
	t.Parallel()

	t.Run("exhaustive check for input error", func(t *testing.T) {
		t.Parallel()
		err := InputError("test", nil, nil)
		assert.False(t, IsUnknownError(err))
		assert.True(t, IsInputError(err))
		assert.False(t, IsCompilationError(err))
		assert.False(t, IsQueueError(err))
		assert.False(t, IsReferenceError(err))
	})

	t.Run("shallow test for other error categories", func(t *testing.T) {
		t.Parallel()

		assert.True(t, IsReferenceError(ReferenceError(nil)))
		assert.True(t, IsCompilationError(CompilationError("test", nil, nil)))
		assert.True(t, IsQueueError(QueueError("test", nil, nil)))
		assert.True(t, IsUnknownError(UnknownError("test", nil, nil)))
	})

	t.Run("non-category errors", func(t *testing.T) {
		t.Parallel()

		assert.False(t, IsUnknownError(io.EOF))
		assert.False(t, IsInputError(io.EOF))
		assert.False(t, IsCompilationError(io.EOF))
		assert.False(t, IsQueueError(io.EOF))
		assert.False(t, IsReferenceError(io.EOF))
	})
}
