package ark

import (
	"fmt"
)

const typ = "Ark"

func getType() string {
	return typ
}

func dereferenceOrZero[T any](v *T) T {
	if v == nil {
		var t T
		return t
	}

	return *v
}

func ptrFromOrZero[T any](v *T) T {
	if v == nil {
		var t T
		return t
	}
	return *v
}

func ptrOf[T any](v T) *T {
	return &v
}

type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

func newPanicErr(info any, stack []byte) error {
	return &panicErr{
		info:  info,
		stack: stack,
	}
}
