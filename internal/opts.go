package internal

import "github.com/5idu/aurora/utils"

// Options block for the server
type Options struct {
	Host     string
	Port     int
	ProfPort int
	HTTPPort int

	MaxPayloadSize int64

	EtcdAddr []string
}

func DefaultOptions() *Options {
	return &Options{
		Host:           utils.GetLocalIpV4(),
		Port:           6666,
		ProfPort:       6001,
		HTTPPort:       6002,
		MaxPayloadSize: 1024 * 1024, // 1MB
		EtcdAddr:       []string{"http://127.0.0.1:2379"},
	}
}
