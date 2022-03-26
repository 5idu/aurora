package internal

import "github.com/pkg/errors"

var (
	ErrMaxPayload     = errors.New("maximum payload exceeded")
	ErrMaxControlLine = errors.New("maximum control line exceeded")
)
