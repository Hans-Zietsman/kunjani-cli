package client

import "fmt"

type GenericError struct{ Msg string }

func (e *GenericError) Error() string { return e.Msg }

type AuthError struct{ Msg string }

func (e *AuthError) Error() string { return e.Msg }

type NotFoundError struct{ Msg string }

func (e *NotFoundError) Error() string { return e.Msg }

type ValidationError struct{ Errors []string }

func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0]
	}
	return fmt.Sprintf("%d validation errors", len(e.Errors))
}
