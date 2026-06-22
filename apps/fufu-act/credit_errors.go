package activityapp

import "errors"

type permanentCreditError struct {
	err error
}

func (e permanentCreditError) Error() string {
	if e.err == nil {
		return "permanent credit failure"
	}
	return e.err.Error()
}

func (e permanentCreditError) Unwrap() error {
	return e.err
}

func newPermanentCreditError(err error) error {
	if err == nil {
		return nil
	}
	return permanentCreditError{err: err}
}

func isPermanentCreditError(err error) bool {
	var target permanentCreditError
	return errors.As(err, &target)
}
