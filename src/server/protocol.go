package server

type MsgType string

const (
	TypeSend       MsgType = "send"
	TypeSendAck    MsgType = "send_ack"
	TypeDeliver    MsgType = "deliver"
	TypeDeliverAck MsgType = "deliver_ack"
	TypeSync       MsgType = "sync"
)

// Message is a single-line JSON protocol message.
type Message struct {
	Type        MsgType `json:"type"`
	ClientMsgID string  `json:"client_msg_id,omitempty"`
	ServerMsgID string  `json:"server_msg_id,omitempty"`
	ChatID      string  `json:"chat_id,omitempty"`
	From        string  `json:"from,omitempty"`
	To          string  `json:"to,omitempty"`
	Seq         uint64  `json:"seq,omitempty"`
	Body        string  `json:"body,omitempty"`
	Ts          int64   `json:"ts,omitempty"`
}
