package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/bt-smart/tuya-pulsar-sdk-go/pkg/btlog"
	"go.uber.org/zap"

	pulsar "github.com/bt-smart/tuya-pulsar-sdk-go"
	"github.com/bt-smart/tuya-pulsar-sdk-go/pkg/tyutils"
)

func main() {
	// SetInternalLogLevel(logrus.DebugLevel)
	accessID := "accessID"
	accessKey := "accessKey"
	topic := pulsar.TopicForAccessID(accessID)

	// create client
	cfg := pulsar.ClientConfig{
		PulsarAddr: pulsar.PulsarAddrCN,
	}
	c := pulsar.NewClient(cfg)

	// create consumer
	csmCfg := pulsar.ConsumerConfig{
		Topic: topic,
		Auth:  pulsar.NewAuthProvider(accessID, accessKey),
	}
	csm, _ := c.NewConsumer(csmCfg)

	// handle message
	csm.ReceiveAndHandle(context.Background(), &helloHandler{AesSecret: accessKey[8:24]})
}

type helloHandler struct {
	AesSecret string
}

func (h *helloHandler) HandlePayload(ctx context.Context, msg pulsar.Message, payload []byte) error {
	btlog.Logger.Info("payload preview", zap.String("payload", string(payload)))

	decryptModel := msg.Properties()["em"]
	// let's decode the payload with AES
	m := map[string]interface{}{}
	err := json.Unmarshal(payload, &m)
	if err != nil {
		btlog.Logger.Error("json unmarshal failed", zap.String("err", err.Error()))
		return nil
	}
	bs := m["data"].(string)
	de, err := base64.StdEncoding.DecodeString(string(bs))
	if err != nil {
		btlog.Logger.Error("base64 decode failed", zap.String("err", err.Error()))
		return nil
	}
	decode, err := tyutils.Decrypt(de, []byte(h.AesSecret), decryptModel)
	btlog.Logger.Info("aes decode", zap.ByteString("decode payload", decode))

	return nil
}
