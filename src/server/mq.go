package server

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

type MQMode string

const (
	MQModeLocal MQMode = "local"
	MQModeRedis MQMode = "redis"
	MQModeKafka MQMode = "kafka"
	MQModeDual  MQMode = "dual"
)

type MessageBus struct {
	mode MQMode

	redisClient *redis.Client
	redisList   string

	kafkaWriter *kafka.Writer
	kafkaReader *kafka.Reader

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewMessageBusFromEnv() *MessageBus {
	mode := MQMode(strings.ToLower(strings.TrimSpace(os.Getenv("IM_MQ_MODE"))))
	if mode == "" {
		mode = MQModeLocal
	}
	if mode != MQModeLocal && mode != MQModeRedis && mode != MQModeKafka && mode != MQModeDual {
		mode = MQModeLocal
	}

	ctx, cancel := context.WithCancel(context.Background())
	bus := &MessageBus{
		mode:   mode,
		ctx:    ctx,
		cancel: cancel,
	}

	if mode == MQModeRedis || mode == MQModeDual {
		addr := strings.TrimSpace(os.Getenv("IM_REDIS_ADDR"))
		if addr == "" {
			addr = "127.0.0.1:6379"
		}
		bus.redisList = strings.TrimSpace(os.Getenv("IM_REDIS_LIST_KEY"))
		if bus.redisList == "" {
			bus.redisList = "im:deliver:q"
		}
		bus.redisClient = redis.NewClient(&redis.Options{
			Addr: addr,
		})
	}

	if mode == MQModeKafka || mode == MQModeDual {
		raw := strings.TrimSpace(os.Getenv("IM_KAFKA_BROKERS"))
		var brokers []string
		for _, b := range strings.Split(raw, ",") {
			v := strings.TrimSpace(b)
			if v != "" {
				brokers = append(brokers, v)
			}
		}
		if len(brokers) == 0 {
			log.Printf("[MQ] kafka mode enabled but IM_KAFKA_BROKERS is empty, fallback to local publish")
		} else {
			topic := strings.TrimSpace(os.Getenv("IM_KAFKA_TOPIC"))
			if topic == "" {
				topic = "im-deliver"
			}
			groupID := strings.TrimSpace(os.Getenv("IM_KAFKA_GROUP_ID"))
			if groupID == "" {
				groupID = "im-gate"
			}
			bus.kafkaWriter = &kafka.Writer{
				Addr:         kafka.TCP(brokers...),
				Topic:        topic,
				Balancer:     &kafka.LeastBytes{},
				RequiredAcks: kafka.RequireOne,
			}
			bus.kafkaReader = kafka.NewReader(kafka.ReaderConfig{
				Brokers:  brokers,
				Topic:    topic,
				GroupID:  groupID,
				MinBytes: 1,
				MaxBytes: 10e6,
			})
		}
	}

	log.Printf("[MQ] mode=%s", bus.mode)
	return bus
}

func (b *MessageBus) StartConsumers(push func(string)) {
	if b == nil {
		return
	}
	if b.mode == MQModeRedis || b.mode == MQModeDual {
		b.startRedisConsumer(push)
	}
	if b.mode == MQModeKafka || b.mode == MQModeDual {
		b.startKafkaConsumer(push)
	}
}

func (b *MessageBus) startRedisConsumer(push func(string)) {
	if b.redisClient == nil || b.redisList == "" {
		return
	}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-b.ctx.Done():
				return
			default:
			}
			out, err := b.redisClient.BRPop(b.ctx, 5*time.Second, b.redisList).Result()
			if err != nil {
				if err == redis.Nil || b.ctx.Err() != nil {
					continue
				}
				time.Sleep(500 * time.Millisecond)
				continue
			}
			if len(out) == 2 && strings.TrimSpace(out[1]) != "" {
				push(strings.TrimSpace(out[1]))
			}
		}
	}()
}

func (b *MessageBus) startKafkaConsumer(push func(string)) {
	if b.kafkaReader == nil {
		return
	}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			msg, err := b.kafkaReader.ReadMessage(b.ctx)
			if err != nil {
				if b.ctx.Err() != nil {
					return
				}
				time.Sleep(500 * time.Millisecond)
				continue
			}
			payload := strings.TrimSpace(string(msg.Value))
			if payload != "" {
				push(payload)
			}
		}
	}()
}

func (b *MessageBus) Publish(serverMsgID string, fallback func(string)) {
	id := strings.TrimSpace(serverMsgID)
	if id == "" {
		return
	}
	if b == nil || b.mode == MQModeLocal {
		fallback(id)
		return
	}

	published := false
	if (b.mode == MQModeRedis || b.mode == MQModeDual) && b.redisClient != nil && b.redisList != "" {
		if err := b.redisClient.LPush(b.ctx, b.redisList, id).Err(); err == nil {
			published = true
		}
	}
	if (b.mode == MQModeKafka || b.mode == MQModeDual) && b.kafkaWriter != nil {
		if err := b.kafkaWriter.WriteMessages(b.ctx, kafka.Message{Value: []byte(id)}); err == nil {
			published = true
		}
	}
	if !published {
		fallback(id)
	}
}

func (b *MessageBus) Close() {
	if b == nil {
		return
	}
	b.cancel()
	if b.kafkaReader != nil {
		_ = b.kafkaReader.Close()
	}
	if b.kafkaWriter != nil {
		_ = b.kafkaWriter.Close()
	}
	if b.redisClient != nil {
		_ = b.redisClient.Close()
	}
	b.wg.Wait()
}
