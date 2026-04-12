package server

import "tet/src/protocol"

type MsgType = protocol.MsgType
type Message = protocol.Message

const (
	TypeSend       = protocol.TypeSend
	TypeSendAck    = protocol.TypeSendAck
	TypeDeliver    = protocol.TypeDeliver
	TypeDeliverAck = protocol.TypeDeliverAck
	TypeReadAck    = protocol.TypeReadAck
	TypeSync       = protocol.TypeSync
)
