package internal

import (
	"net"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type client struct {
	cid uint64

	srv     *Server
	conn    net.Conn
	startAt time.Time

	parseState
}

func (c *client) reedLoop() {
	defer c.conn.Close()

	for {
		buf := make([]byte, c.srv.info.MaxPayload)
		n, err := c.conn.Read(buf)
		if err != nil {
			log.Error().Msg(errors.WithMessage(err, "failed to read from connection").Error())
			return
		}

		if err := c.parse(buf[:n]); err != nil {
			log.Error().Msg(errors.WithMessage(err, "failed to parse message").Error())
			return
		}
	}
}

func (c *client) close() {
	c.conn.Close()
}

// TODO:
func (c *client) processMsg(msg []byte) {
	log.Info().Msgf("processMsg: %s", msg)

	c.sendOkMsg()

	log.Info().Msgf("write to subscriber: %s", msg)
}
