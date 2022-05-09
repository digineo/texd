package tex

import (
	"encoding/json"
	"fmt"
)

type errCategory uint8

const (
	inputErr errCategory = iota + 1
	compilationErr
	queueErr
	referenceErr
)

func (cat errCategory) String() string {
	switch cat {
	case inputErr:
		return "input"
	case compilationErr:
		return "compilation"
	case queueErr:
		return "queue"
	case referenceErr:
		return "reference"
	default:
		return "unknown"
	}
}

type KV map[string]interface{}

type ErrWithCategory struct {
	cat     errCategory
	message string
	cause   error
	extra   KV
}

// ExtendError adds the given extra ke/value pairs to err, if err is a
// ErrWithCategory. If extra is empty or err is another type of error,
// this function does nothing.
func ExtendError(err error, extra KV) {
	if len(extra) == 0 {
		return
	}
	if catErr, ok := err.(*ErrWithCategory); ok {
		if catErr.extra == nil {
			catErr.extra = extra
			return
		}
		for k, v := range extra {
			catErr.extra[k] = v
		}
	}
}

func newCategoryError(cat errCategory, message string, cause error, extra KV) error {
	return &ErrWithCategory{cat: cat, message: message, cause: cause, extra: extra}
}

func InputError(message string, cause error, extra KV) error {
	return newCategoryError(inputErr, message, cause, extra)
}

func ReferenceError(references []string) error {
	return newCategoryError(referenceErr, "unknown file references", nil, KV{
		"references": references,
	})
}

func CompilationError(message string, cause error, extra KV) error {
	return newCategoryError(compilationErr, message, cause, extra)
}

func QueueError(message string, cause error, extra KV) error {
	return newCategoryError(queueErr, message, cause, extra)
}

func UnknownError(message string, cause error, extra KV) error {
	return newCategoryError(0, message, cause, extra)
}

func errorIs(err error, cat errCategory) bool {
	if catErr, ok := err.(*ErrWithCategory); ok {
		return catErr.cat == cat
	}
	return false
}

func IsUnknownError(err error) bool     { return errorIs(err, 0) }
func IsInputError(err error) bool       { return errorIs(err, inputErr) }
func IsCompilationError(err error) bool { return errorIs(err, compilationErr) }
func IsQueueError(err error) bool       { return errorIs(err, queueErr) }
func IsReferenceError(err error) bool   { return errorIs(err, referenceErr) }

func (err *ErrWithCategory) Error() string {
	if err.cause == nil {
		return err.message
	}
	return fmt.Sprintf("%s: %v", err.message, err.cause)
}

func (err *ErrWithCategory) Unwrap() error {
	return err.cause
}

func (err *ErrWithCategory) Extra() KV {
	return err.extra
}

func (err *ErrWithCategory) MarshalJSON() ([]byte, error) {
	data := KV{
		"error":    err.message, // omit cause, could leak internal data
		"category": err.cat.String(),
	}
	for k, v := range err.extra {
		if k != "error" && k != "category" {
			data[k] = v
		}
	}
	return json.Marshal(data)
}
