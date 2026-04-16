package server

import (
	"fmt"
	"log"
	"strings"
	"time"
)

func (s *Server) HandleMessage(user interface{ GetName() string; SendJSON(*Message) }, m *Message) {
	switch m.Type {
	case TypeSend:
		if m.From == "" {
			m.From = user.GetName()
		}
		s.HandleClientSend(user, m)
	case TypeDeliverAck:
		if m.ServerMsgID != "" {
			s.logic.HandleDeliverAck(user.GetName(), m.ServerMsgID)
		}
	case TypeReadAck:
		if m.ServerMsgID != "" {
			s.logic.HandleReadAck(user.GetName(), m.ServerMsgID)
		}
	}
}

func (s *Server) HandleClientSend(u interface{ GetName() string; SendJSON(*Message) }, req *Message) {
	if strings.TrimSpace(req.From) == "" {
		req.From = u.GetName()
	}
	if strings.TrimSpace(req.ClientMsgID) == "" {
		req.ClientMsgID = fmt.Sprintf("c-%d", time.Now().UnixNano())
	}
	var recipients []string
	if req.To != "" {
		recipients = append(recipients, req.To)
	} else {
		s.MapLock.RLock()
		for name := range s.OnlineMap {
			if name != u.GetName() {
				recipients = append(recipients, name)
			}
		}
		s.MapLock.RUnlock()
	}
	msg, existing, err := s.logic.ProcessSend(req, recipients)
	if err != nil {
		log.Printf("[HandleClientSend] err: %v", err)
		u.SendJSON(&Message{Type: TypeSendAck, ClientMsgID: req.ClientMsgID})
		return
	}
	if existing != nil {
		u.SendJSON(&Message{Type: TypeSendAck, ClientMsgID: req.ClientMsgID, ServerMsgID: existing.ServerMsgID, Seq: existing.Seq})
		return
	}
	u.SendJSON(&Message{Type: TypeSendAck, ClientMsgID: req.ClientMsgID, ServerMsgID: msg.ServerMsgID, Seq: msg.Seq})
	s.EnqueueServerMsg(msg.ServerMsgID)
}
