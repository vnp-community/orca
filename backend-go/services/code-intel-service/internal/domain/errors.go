package domain

import "errors"

var ErrResourceExhausted = errors.New("resource exhausted")
var ErrUnavailable = errors.New("unavailable")
