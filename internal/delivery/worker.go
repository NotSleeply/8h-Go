package delivery

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"goim/internal/protocol"
	"goim/internal/store"
	"goim/internal/svcapi"
)

type Worker struct {
	store           store.Store
	deliverQueue    chan string
	deliverInFlight map[string]struct{}
	inFlightMu      sync.Mutex
	maxRetry        int
	retryBaseDelay  time.Duration
	onlineMap       func() map[string]svcapi.Sender
}

func NewWorker(st store.Store, maxRetry int, retryBaseDelay time.Duration, onlineMapFunc func() map[string]svcapi.Sender) *Worker {
	return &Worker{
		store:           st,
		deliverQueue:    make(chan string, 1024),
		deliverInFlight: make(map[string]struct{}),
		maxRetry:        maxRetry,
		retryBaseDelay:  retryBaseDelay,
		onlineMap:       onlineMapFunc,
	}
}

func (w *Worker) EnqueueServerMsg(serverMsgID string) {
	if serverMsgID == "" {
		return
	}
	w.inFlightMu.Lock()
	if _, ok := w.deliverInFlight[serverMsgID]; ok {
		w.inFlightMu.Unlock()
		return
	}
	w.deliverInFlight[serverMsgID] = struct{}{}
	w.inFlightMu.Unlock()

	select {
	case w.deliverQueue <- serverMsgID:
	default:
		w.inFlightMu.Lock()
		delete(w.deliverInFlight, serverMsgID)
		w.inFlightMu.Unlock()
	}
}

func (w *Worker) EnqueuePendingForUser(username string, limit int) {
	ids := w.store.ListPendingServerMsgIDsByUser(username, limit)
	for _, id := range ids {
		w.EnqueueServerMsg(id)
	}
}

func (w *Worker) RecoverPendingDeliveries(limit int) {
	ids := w.store.RecoverPendingServerMsgIDs(limit)
	for _, id := range ids {
		w.EnqueueServerMsg(id)
	}
}

func (w *Worker) Start() {
	go w.deliverWorker()
	go w.retryWorker()
}

func (w *Worker) deliverWorker() {
	for serverMsgID := range w.deliverQueue {
		func(id string) {
			defer func() {
				w.inFlightMu.Lock()
				delete(w.deliverInFlight, id)
				w.inFlightMu.Unlock()
			}()
			msg := w.store.GetMessageByServerID(id)
			if msg == nil {
				return
			}
			recips := w.store.GetRecipients(id)
			if msg.To != "" {
				seen := false
				for _, r := range recips {
					if r == msg.To {
						seen = true
						break
					}
				}
				if !seen {
					recips = append(recips, msg.To)
				}
			}
			onlineMap := w.onlineMap()
			for _, r := range recips {
				sender, ok := onlineMap[r]
				if !ok {
					dead, err := w.store.ScheduleRetry(id, r, errors.New("recipient offline"), w.maxRetry, w.retryBaseDelay)
					if err != nil {
						fmt.Println("[DeliverWorker] schedule retry failed:", err)
					}
					if dead {
						fmt.Printf("[DeliverWorker] dead-letter: msg=%s to=%s\n", id, r)
					}
					continue
				}
				deliver := &protocol.Message{
					Type:        protocol.TypeDeliver,
					ServerMsgID: id,
					ChatID:      msg.ChatID,
					From:        msg.From,
					To:          r,
					Seq:         msg.Seq,
					Body:        msg.Body,
					Ts:          msg.Ts,
				}
				if sender != nil {
					fmt.Printf("[DeliverWorker] deliver msg=%s to=%s online=true\n", id, r)
					sender.SendJSON(deliver)
				}
				_ = w.store.MarkDeliverySent(id, r, nil)
			}
		}(serverMsgID)
	}
}

func (w *Worker) retryWorker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		ids := w.store.GetDueRetryServerMsgIDs(500)
		for _, id := range ids {
			w.EnqueueServerMsg(id)
		}
	}
}
