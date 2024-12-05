package server

import "errors"

type HttpError struct {
	error
	ResponseCode int
}

var _ error = (*HttpError)(nil)

func NewHttpError(responseCode int, message string) *HttpError {
	return &HttpError{
		error:        errors.New(message),
		ResponseCode: responseCode,
	}
}
