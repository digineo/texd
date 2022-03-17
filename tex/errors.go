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

func (err *ErrWithCategory) Error() string {
	if err.cause == nil {
		return err.message
	}
	return fmt.Sprintf("%s: %v", err.message, err.cause)
}

func (err *ErrWithCategory) Unwrap() error {
	return err.cause
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
