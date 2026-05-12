package otsql

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ErrToCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	if s, ok := status.FromError(err); ok {
		return s.Code()
	}
	return codes.Unknown
}
