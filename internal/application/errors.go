package application

import "errors"

type PermanentError struct{ Err error }

func (e *PermanentError) Error() string { return "erro permanente: " + e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

func Permanent(err error) error { return &PermanentError{Err: err} }

func IsPermanent(err error) bool {
	var pe *PermanentError
	return errors.As(err, &pe)
}
