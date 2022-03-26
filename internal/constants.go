package internal

import "runtime"

// server constants
const (
	Version       = "0.0.1"
	Proto         = "1"
	EtcdParentKey = "/goqueue/nodes/"
)

var GoVersion = runtime.Version()

// common constants
const (
	CRLF = "\r\n"
)

// message constants
const (
	MaxMessageSize = 1024
	MaxPubArgs     = 2
	// MaxControlLineSize is the maximum allowed protocol control line size.
	// 1k should be plenty since payloads sans connect string are separate
	MaxControlLineSize = 1024
)
