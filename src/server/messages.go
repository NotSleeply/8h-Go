package server

import (
	"fmt"
	"strings"
	"time"
)

// HandleMessage dispatches incoming protocol messages from clients.
func (s *Server) HandleMessage(user *User, m *Message) {
	switch m.Type {
	case TypeSend:
		if m.From == "" {
			m.From = user.Name
		}
		s.HandleClientSend(user, m)
	case TypeDeliverAck:
		s.HandleDeliverAck(user, m)
	case TypeReadAck:
		s.HandleReadAck(user, m)
	default:
		// ignore other types for now
	}
}

// HandleClientSend handles a client's send request: dedupe, persist in-memory and enqueue for delivery.
func (s *Server) HandleClientSend(u *User, req *Message) {
	if strings.TrimSpace(req.From) == "" {
		req.From = u.Name
	}
	if strings.TrimSpace(req.ClientMsgID) == "" {
		req.ClientMsgID = fmt.Sprintf("c-%d", time.Now().UnixNano())
	}

	var recipients []string
	// recipients: private or broadcast to all online except sender
	if req.To != "" {
		recipients = append(recipients, req.To)
	} else {
		s.MapLock.RLock()
		for _, user := range s.OnlineMap {
			if user.Name == u.Name {
				continue
			}
			recipients = append(recipients, user.Name)
		}
		s.MapLock.RUnlock()
	}

	msg, existing, err := s.logic.ProcessSend(req, recipients)
	if err != nil {
		u.SendMsg("消息存储失败，请稍后重试。")
		return
	}
	if existing != nil {
		ack := &Message{Type: TypeSendAck, ClientMsgID: req.ClientMsgID, ServerMsgID: existing.ServerMsgID, Seq: existing.Seq}
		u.SendJSON(ack)
		return
	}

	// reply send_ack
	ack := &Message{Type: TypeSendAck, ClientMsgID: req.ClientMsgID, ServerMsgID: msg.ServerMsgID, Seq: msg.Seq}
	u.SendJSON(ack)

	// enqueue for delivery
	s.EnqueueServerMsg(msg.ServerMsgID)
}

// HandleDeliverAck marks a delivery as acknowledged by recipient.
func (s *Server) HandleDeliverAck(u *User, m *Message) {
	if m.ServerMsgID == "" {
		return
	}
	s.logic.HandleDeliverAck(u.Name, m.ServerMsgID)
}

func (s *Server) HandleReadAck(u *User, m *Message) {
	if m.ServerMsgID == "" {
		return
	}
	s.logic.HandleReadAck(u.Name, m.ServerMsgID)
}
