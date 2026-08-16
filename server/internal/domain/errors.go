package domain

import (
	"errors"
	"fmt"
)

type Kind int

const (
	KindInternal Kind = iota
	KindInvalid
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindRateLimited
)

type Error struct {
	Kind Kind
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

func Invalid(format string, a ...any) *Error {
	return &Error{Kind: KindInvalid, Msg: fmt.Sprintf(format, a...)}
}

func Unauthorized(msg string) *Error {
	return &Error{Kind: KindUnauthorized, Msg: msg}
}

func Forbidden(msg string) *Error {
	return &Error{Kind: KindForbidden, Msg: msg}
}

func NotFound(what string) *Error {
	return &Error{Kind: KindNotFound, Msg: what + " not found"}
}

func Conflict(msg string) *Error {
	return &Error{Kind: KindConflict, Msg: msg}
}

func RateLimited(msg string) *Error {
	return &Error{Kind: KindRateLimited, Msg: msg}
}

func Internal(err error) *Error {
	return &Error{Kind: KindInternal, Msg: "internal error", Err: err}
}

func KindOf(err error) Kind {
	if e, ok := errors.AsType[*Error](err); ok {
		return e.Kind
	}
	return KindInternal
}
