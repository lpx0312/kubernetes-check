package errors

import (
    "errors"
    "testing"
)

func TestAppError_Error(t *testing.T) {
    err := &AppError{
        Code:    ErrConfigLoad,
        Op:      "TestOp",
        Err:     errors.New("original error"),
        Message: "test message",
    }

    got := err.Error()
    want := "[1003] TestOp: test message"

    if got != want {
        t.Errorf("Error() = %q, want %q", got, want)
    }
}

func TestAppError_Unwrap(t *testing.T) {
    original := errors.New("original")
    err := &AppError{
        Code: ErrConfigLoad,
        Op:   "TestOp",
        Err:  original,
    }

    got := errors.Unwrap(err)
    if got != original {
        t.Errorf("Unwrap() = %v, want %v", got, original)
    }
}

func TestNewAppError(t *testing.T) {
    original := errors.New("test")
    err := NewAppError(ErrConfigLoad, "TestOp", original, "user message")

    if err.Code != ErrConfigLoad {
        t.Errorf("Code = %v, want %v", err.Code, ErrConfigLoad)
    }
    if err.Op != "TestOp" {
        t.Errorf("Op = %q, want %q", err.Op, "TestOp")
    }
    if err.Err != original {
        t.Errorf("Err = %v, want %v", err.Err, original)
    }
    if err.Message != "user message" {
        t.Errorf("Message = %q, want %q", err.Message, "user message")
    }
}
