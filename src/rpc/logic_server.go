package rpc

import (
	"context"
	"fmt"
	"strings"

	"tet/src/rpc/pb"
	"tet/src/server"
)

type LogicGRPCServer struct {
	pb.UnimplementedLogicServiceServer
	Srv *server.Server
}

func NewLogicGRPCServer(s *server.Server) *LogicGRPCServer {
	return &LogicGRPCServer{Srv: s}
}

func (g *LogicGRPCServer) SendMessage(ctx context.Context, req *pb.SendMessageReq) (*pb.SendMessageResp, error) {
	_ = ctx
	if g == nil || g.Srv == nil {
		return nil, fmt.Errorf("server not ready")
	}
	from := strings.TrimSpace(req.GetFrom())
	if from == "" {
		return nil, fmt.Errorf("from is required")
	}

	recipients := []string{}
	to := strings.TrimSpace(req.GetTo())
	if to != "" {
		recipients = append(recipients, to)
	} else {
		g.Srv.MapLock.RLock()
		for name := range g.Srv.OnlineMap {
			if name == from {
				continue
			}
			recipients = append(recipients, name)
		}
		g.Srv.MapLock.RUnlock()
	}

	m := &server.Message{
		Type:        server.TypeSend,
		ClientMsgID: strings.TrimSpace(req.GetClientMsgId()),
		ChatID:      strings.TrimSpace(req.GetChatId()),
		From:        from,
		To:          to,
		Body:        req.GetBody(),
	}
	saved, existing, err := g.Srv.Logic().ProcessSend(m, recipients)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return &pb.SendMessageResp{
			ServerMsgId: existing.ServerMsgID,
			Seq:         existing.Seq,
			Deduped:     true,
		}, nil
	}
	g.Srv.EnqueueServerMsg(saved.ServerMsgID)
	return &pb.SendMessageResp{
		ServerMsgId: saved.ServerMsgID,
		Seq:         saved.Seq,
		Deduped:     false,
	}, nil
}

func (g *LogicGRPCServer) AckDelivery(ctx context.Context, req *pb.AckDeliveryReq) (*pb.AckDeliveryResp, error) {
	_ = ctx
	if g == nil || g.Srv == nil {
		return nil, fmt.Errorf("server not ready")
	}
	g.Srv.Logic().HandleDeliverAck(strings.TrimSpace(req.GetTo()), strings.TrimSpace(req.GetServerMsgId()))
	return &pb.AckDeliveryResp{Ok: true}, nil
}

func (g *LogicGRPCServer) AckRead(ctx context.Context, req *pb.AckReadReq) (*pb.AckReadResp, error) {
	_ = ctx
	if g == nil || g.Srv == nil {
		return nil, fmt.Errorf("server not ready")
	}
	g.Srv.Logic().HandleReadAck(strings.TrimSpace(req.GetTo()), strings.TrimSpace(req.GetServerMsgId()))
	return &pb.AckReadResp{Ok: true}, nil
}
