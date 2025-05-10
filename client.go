package pulsar

import (
	"context"
	"fmt"
	"github.com/bt-smart/tuya-pulsar-sdk-go/pkg/btlog"
	"go.uber.org/zap"
	"strings"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
)

const (
	PulsarAddrCN  = "pulsar+ssl://mqe.tuyacn.com:7285"
	PulsarAddrEU  = "pulsar+ssl://mqe.tuyaeu.com:7285"
	PulsarAddrUS  = "pulsar+ssl://mqe.tuyaus.com:7285"
	PulsarAddrIND = "pulsar+ssl://mqe.tuyain.com:7285"
)

type Message = pulsar.Message

type Client interface {
	NewConsumer(config ConsumerConfig) (Consumer, error)
}

type ProducerMessage struct {
	Payload []byte
	Key     string
}

type Consumer interface {
	ReceiveAndHandle(ctx context.Context, handler PayloadHandlerV2)
	Close() error
}

type PayloadHandlerV2 interface {
	HandlePayload(ctx context.Context, msg Message, payload []byte) error
}

type clientImpl struct {
	cli       pulsar.Client
	clientCfg ClientConfig
}

type ClientConfig struct {
	PulsarAddr string
	Auth       interface{}
}

type ConsumerConfig struct {
	Topic        string
	Subscription string
	Auth         interface{}
}

func NewClient(cfg ClientConfig) Client {
	return newClientV2(cfg)
}

func newClientV2(cfg ClientConfig) Client {
	return clientImpl{clientCfg: cfg}
}

func (c clientImpl) NewConsumer(config ConsumerConfig) (Consumer, error) {
	if config.Auth != nil || c.cli == nil {
		client, err := pulsar.NewClient(pulsar.ClientOptions{
			TLSAllowInsecureConnection: true,
			URL:                        c.clientCfg.PulsarAddr,
			Authentication:             config.Auth,
		})
		if err != nil {
			btlog.Logger.Error("create clientImpl failed", zap.String("err", err.Error()), zap.Any("config", c.clientCfg))
			return nil, err
		}
		btlog.Logger.Info("create clientImpl success", zap.Any("config", c.clientCfg))
		c.cli = client
	}
	consumer, err := c.cli.Subscribe(pulsar.ConsumerOptions{
		Topic:            config.Topic,
		SubscriptionName: subscriptionName(config.Topic),
		Type:             pulsar.Failover,
	})
	if err != nil {
		btlog.Logger.Error("create consumer failed", zap.String("err", err.Error()), zap.Any("config", config))
		return nil, err
	}
	btlog.Logger.Info("create consumer success", zap.Any("config", config))
	return consumerV2{consumer}, nil
}

type consumerV2 struct {
	consumer pulsar.Consumer
}

func (c consumerV2) ReceiveAndHandle(ctx context.Context, handler PayloadHandlerV2) {
	for i := 0; i < 10; i++ {
		go func() {
			for {
				msg, err := c.consumer.Receive(context.Background())
				if err != nil {
					btlog.Logger.Error("consumer receive failed", zap.Error(err), zap.Any("consumer", c.consumer))
					continue
				}
				start := time.Now()
				id := MsgId(msg.ID())
				btlog.Logger.Info("consume receive", zap.Any("message", msg))
				err = handler.HandlePayload(ctx, msg, msg.Payload())
				if err != nil {
					btlog.Logger.Warn("consumer HandlePayload failed", zap.Error(err), zap.Any("consumer", c.consumer), zap.Any("msg", msg))
				}
				duration := time.Since(start)
				ackStart := time.Now()
				btlog.Logger.Info("consume handle finish", zap.Any("messageId", id), zap.Any("cost", duration))
				retryCount := 3
				for j := 0; j < retryCount; j++ {
					err := c.consumer.Ack(msg)
					if err != nil {
						btlog.Logger.Warn("ack failed", zap.String("msg", string(msg.Payload())))
						time.Sleep(time.Second)
					} else {
						break
					}
				}
				ackDuration := time.Since(ackStart)
				btlog.Logger.Info("consume ack finish", zap.Any("messageId", id), zap.Any("cost", ackDuration))
			}
		}()
	}
	select {
	case <-ctx.Done():
		return
	}
}

func MsgId(id pulsar.MessageID) string {
	return fmt.Sprintf("%d:%d:%d:%d", id.LedgerID(), id.EntryID(), id.PartitionIdx(), id.BatchIdx())
}

func (c consumerV2) Close() error {
	c.consumer.Close()
	return nil
}

func TopicForAccessID(accessID string) string {
	topic := fmt.Sprintf("persistent://%s/out/event", accessID)
	return topic
}

func subscriptionName(topic string) string {
	return getTenant(topic) + "-sub"
}

func getTenant(topic string) string {
	topic = strings.TrimPrefix(topic, "persistent://")
	end := strings.Index(topic, "/")
	return topic[:end]
}
