package utils

import (
	"net"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

func GetLocalIpV4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatal().Msg(errors.WithMessage(err, "failed get interface addr").Error())
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	log.Fatal().Msg(errors.WithMessage(err, "unable to get ip address").Error())
	return ""
}
