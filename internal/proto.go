package internal

import (
	"fmt"

	aurora_proto "github.com/5idu/aurora/proto"
	"github.com/gogo/protobuf/proto"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

/*
proto description:

Field Delimiters: The fields of protocol messages are delimited by whitespace characters(space) or `\t`(tab). Multiple whitespace characters or `\t` will be treated as a single field delimiter.

Newlines: uses CRLF to terminate protocol messages. This newline sequence is also used to mark the end of the message payload in a PUB or MSG protocol message.
*/

// proto message types
/*
PING\r\n
PUB <subject> <payload_length>\r\n[payload]\r\n
SUB <subject> [queue group]\r\n

+OK\r\n
-ERR <error message>\r\n
*17\r\n[{"name": "bob"}]\r\n
*/
const (
	OP_START = iota

	OP_P
	OP_PI
	OP_PIN
	OP_PING

	OP_PU
	OP_PUB
	OP_PUB_SPACE
	OP_PUB_ARG

	MSG_PAYLOAD
	MSG_END

	OP_S
	OP_SU
	OP_SUB
	OP_SUB_SPACE
	OP_SUB_PAYLOAD

	OP_U
	OP_UN
	OP_UNS
	OP_UNSU
	OP_UNSUB
	OP_UNSUB_SPACE
	OP_UNSUB_PAYLOAD
)

type parseState struct {
	state   int
	as      int
	drop    int
	pa      pubArg
	argBuf  []byte
	msgBuf  []byte
	scratch [MaxControlLineSize]byte
}

type pubArg struct {
	subject []byte // pub subject
	sid     []byte
	szb     []byte // pub payload size buf
	size    int    // pub payload size
}

// parse message
func (c *client) parse(msg []byte) error {
	log.Info().Msgf("received: %s", msg)

	var data aurora_proto.Message
	if err := proto.Unmarshal(msg, &data); err != nil {
		return err
	}
	fmt.Println(data.String())
	fmt.Println(proto.MessageType("aurora.proto.Message11"))
	return nil

	//	var i int
	//	var b byte
	//
	//	for i = 0; i < len(msg); i++ {
	//		b = msg[i]
	//		switch c.state {
	//		case OP_START:
	//			switch b {
	//			case 'P', 'p':
	//				c.state = OP_P
	//			default:
	//				goto parseErr
	//			}
	//		case OP_P:
	//			switch b {
	//			case 'I', 'i':
	//				c.state = OP_PI
	//			case 'U', 'u':
	//				c.state = OP_PU
	//			default:
	//				goto parseErr
	//			}
	//		case OP_PI:
	//			switch b {
	//			case 'N', 'n':
	//				c.state = OP_PIN
	//			default:
	//				goto parseErr
	//			}
	//		case OP_PIN:
	//			switch b {
	//			case 'G', 'g':
	//				c.state = OP_PING
	//			default:
	//				goto parseErr
	//			}
	//		case OP_PING:
	//			switch b {
	//			case '\n':
	//				c.sendPongMsg()
	//				c.drop, c.state = 0, OP_START
	//			}
	//		case OP_PU:
	//			switch b {
	//			case 'B', 'b':
	//				c.state = OP_PUB
	//			default:
	//				goto parseErr
	//			}
	//		case OP_PUB:
	//			switch b {
	//			case ' ', '\t':
	//				c.state = OP_PUB_SPACE
	//			default:
	//				goto parseErr
	//			}
	//		case OP_PUB_SPACE:
	//			switch b {
	//			case ' ', '\t':
	//				continue
	//			default:
	//				c.state = OP_PUB_ARG
	//				c.as = i
	//			}
	//		case OP_PUB_ARG:
	//			switch b {
	//			case '\r':
	//				c.drop = 1
	//			case '\n':
	//				var arg []byte
	//				if c.argBuf != nil {
	//					arg = c.argBuf
	//				} else {
	//					arg = msg[c.as : i-c.drop]
	//				}
	//				if err := c.processPubArg(arg); err != nil {
	//					return err
	//				}
	//				c.drop, c.as, c.state = 0, i+1, MSG_PAYLOAD
	//				// If we don't have a saved buffer then jump ahead with
	//				// the index. If this overruns what is left we fall out
	//				// and process split buffer.
	//				if c.msgBuf == nil {
	//					i = c.as + c.pa.size - len(CRLF)
	//				}
	//			default:
	//				if c.argBuf != nil {
	//					c.argBuf = append(c.argBuf, b)
	//				}
	//			}
	//		case MSG_PAYLOAD:
	//			if c.msgBuf != nil {
	//				// copy as much as we can to the buffer and skip ahead.
	//				toCopy := c.pa.size - len(c.msgBuf)
	//				avail := len(msg) - i
	//				if avail < toCopy {
	//					toCopy = avail
	//				}
	//				if toCopy > 0 {
	//					start := len(c.msgBuf)
	//					// This is needed for copy to work.
	//					c.msgBuf = c.msgBuf[:start+toCopy]
	//					copy(c.msgBuf[start:], msg[i:i+toCopy])
	//					// Update our index
	//					i = (i + toCopy) - 1
	//				} else {
	//					// Fall back to append if needed.
	//					c.msgBuf = append(c.msgBuf, b)
	//				}
	//				if len(c.msgBuf) >= c.pa.size {
	//					c.state = MSG_END
	//				}
	//			} else if i-c.as >= c.pa.size {
	//				c.state = MSG_END
	//			}
	//		case MSG_END:
	//			switch b {
	//			case '\n':
	//				if c.msgBuf != nil {
	//					c.msgBuf = append(c.msgBuf, b)
	//				} else {
	//					c.msgBuf = msg[c.as : i+1]
	//				}
	//				// strict check for proto
	//				if len(c.msgBuf) != c.pa.size+len(CRLF) {
	//					goto parseErr
	//				}
	//				c.processMsg(c.msgBuf)
	//				c.argBuf, c.msgBuf = nil, nil
	//				c.drop, c.as, c.state = 0, i+1, OP_START
	//			default:
	//				if c.msgBuf != nil {
	//					c.msgBuf = append(c.msgBuf, b)
	//				}
	//				continue
	//			}
	//		default:
	//			goto parseErr
	//		}
	//	}
	//
	//	if (c.state == MSG_PAYLOAD || c.state == MSG_END) && c.msgBuf == nil {
	//		// We need to clone the pubArg if it is still referencing the
	//		// read buffer and we are not able to process the msg.
	//		if c.argBuf == nil {
	//			c.clonePubArg()
	//		}
	//
	//		// If we will overflow the scratch buffer, just create a
	//		// new buffer to hold the split message.
	//		if c.pa.size > cap(c.scratch)-len(c.argBuf) {
	//			lrem := len(msg[c.as:])
	//
	//			// Consider it a protocol error when the remaining payload
	//			// is larger than the reported size for PUB. It can happen
	//			// when processing incomplete messages from rogue clients.
	//			if lrem > c.pa.size+len(CRLF) {
	//				goto parseErr
	//			}
	//			c.msgBuf = make([]byte, lrem, c.pa.size+len(CRLF))
	//			copy(c.msgBuf, msg[c.as:])
	//		} else {
	//			c.msgBuf = c.scratch[len(c.argBuf):len(c.argBuf)]
	//			c.msgBuf = append(c.msgBuf, msg[c.as:]...)
	//		}
	//	}
	//
	//	return nil
	//
	//parseErr:
	//	c.sendErrMsg([]byte("unknown protocol operation"))
	//	return errors.New("unknown protocol operation")
}

// clonePubArg is used when the split buffer scenario has the pubArg in the existing read buffer, but
// we need to hold onto it into the next read.
func (c *client) clonePubArg() {
	c.argBuf = c.scratch[:0]
	c.argBuf = append(c.argBuf, c.pa.subject...)
	c.argBuf = append(c.argBuf, c.pa.sid...)
	c.argBuf = append(c.argBuf, c.pa.szb...)

	c.pa.subject = c.argBuf[:len(c.pa.subject)]

	if c.pa.sid != nil {
		c.pa.sid = c.argBuf[len(c.pa.subject) : len(c.pa.subject)+len(c.pa.sid)]
	}

	c.pa.szb = c.argBuf[len(c.pa.subject)+len(c.pa.sid):]
}

func (c *client) processPubArg(arg []byte) error {
	log.Info().Msgf("processPubArg: %s", arg)

	args := splitArg(arg, MaxPubArgs)

	switch len(args) {
	case 2:
		c.pa.subject = args[0]
		c.pa.size = parseSizeFromBuf(args[1])
		c.pa.szb = args[1]
	default:
		return errors.Errorf("processPubArg parse error: '%s'", arg)
	}
	if c.pa.size < 0 {
		return errors.Errorf("processPubArg bad or missing payload size: '%s'", arg)
	}
	maxPayloadSize := c.srv.opts.MaxPayloadSize
	if maxPayloadSize > 0 && int64(c.pa.size) > maxPayloadSize {
		c.sendErrMsg([]byte("maximum payload size exceeded"))
		return ErrMaxPayload
	}

	if !IsValidSubject(string(c.pa.subject)) {
		c.sendErrMsg([]byte("invalid subject"))
	}
	return nil
}

func splitArg(arg []byte, argsLen int) [][]byte {
	args := make([][]byte, 0, argsLen)
	start := -1
	for i, b := range arg {
		switch b {
		case ' ', '\t', '\r', '\n':
			if start >= 0 {
				args = append(args, arg[start:i])
				start = -1
			}
		default:
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		args = append(args, arg[start:])
	}
	return args
}

func (c *client) send(msg []byte) {
	c.conn.Write(msg)
}

func (c *client) sendPongMsg() {
	msg := fmt.Sprintf("+%s%s", "PONG", CRLF)
	c.send([]byte(msg))
}

func (c *client) sendOkMsg() {
	msg := fmt.Sprintf("%s%s", "+OK", CRLF)
	c.send([]byte(msg))
}

func (c *client) sendErrMsg(payload []byte) {
	msg := fmt.Sprintf("%s %s%s", "-ERR", payload, CRLF)
	c.send([]byte(msg))
}
