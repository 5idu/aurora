package utils

import (
	"github.com/lithammer/shortuuid/v4"
)

func GenRandomId() string {
	return shortuuid.New()
}
