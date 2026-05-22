package service

import (
	"errors"
	"fmt"
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrConversationCancelled = errors.New("conversation is cancelled")
	ErrProviderNotFound      = errors.New("provider not found")
)

type StreamError struct {
	Provider string
	Model    string
	Cause    error
}

func (e *StreamError) Error() string {
	return fmt.Sprintf("[%s/%s] stream failed: %v", e.Provider, e.Model, e.Cause)
}

func (e *StreamError) Unwrap() error { return e.Cause }

type InputError struct {
	Field   string
	Message string
}

func (e *InputError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}
