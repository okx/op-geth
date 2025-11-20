package kafka

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/ethereum/go-ethereum/log"
)

type rawConsumerGroupHandler struct {
	ctx         context.Context
	rawMsgsChan chan *sarama.ConsumerMessage
	errorChan   chan error
}

func (h *rawConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *rawConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *rawConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	log.Info(fmt.Sprintf("[Realtime] Starting raw kafka consumption. topic: %s, partition: %d, offset: %d", claim.Topic(), claim.Partition(), claim.InitialOffset()))

	for {
		select {
		case <-h.ctx.Done():
			err := fmt.Errorf("context cancelled - stopping raw consume claim")
			h.errorChan <- err
			return err
		case msg, ok := <-claim.Messages():
			if !ok {
				log.Debug("[Realtime] raw kafka consumer failed to get claim messages")
				continue
			}

			select {
			case h.rawMsgsChan <- msg:
				session.MarkMessage(msg, "")
			case <-h.ctx.Done():
				err := fmt.Errorf("context cancelled - stopping raw consume claim")
				h.errorChan <- err
				return err
			}
		}
	}
}

// ConsumeRawKafka consumes raw ConsumerMessages from Kafka topics
func (client *KafkaConsumer) ConsumeRawKafka(ctx context.Context, rawMsgsChan chan *sarama.ConsumerMessage, errorChan chan error) {
	handler := &rawConsumerGroupHandler{
		ctx:         ctx,
		rawMsgsChan: rawMsgsChan,
		errorChan:   errorChan,
	}

	topics := []string{client.config.TxTopic, client.config.BlockTopic, client.config.ErrorTopic}
	err := client.consumer.Consume(ctx, topics, handler)
	if err != nil {
		errorChan <- fmt.Errorf("ConsumeRawKafka error: %v", err)
		return
	}
}
