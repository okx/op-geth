package streamclient

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/realtime/kafka"
	kafkaTypes "github.com/ethereum/go-ethereum/realtime/kafka/types"
	"github.com/ethereum/go-ethereum/realtime/relayer/streamer"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

type StreamClientInterface interface {
	Start() error
	Stop() error
	HandleRestart() error
	SendHeartbeat()
	readMessage() (*streamer.Message, error)
	ConsumeRealtime(blockMsgsChan chan realtimeTypes.BlockInfo, txMsgsChan chan kafkaTypes.TransactionMessage, errorMsgsChan chan kafkaTypes.ErrorTriggerMessage, errorChan chan error)
}

type StreamClient struct {
	ctx         context.Context
	cfg         *StreamClientConfig
	tlsConfig   *tls.Config
	readyFlag   atomic.Bool
	resetFlag   atomic.Bool
	connMutex   sync.RWMutex
	conn        net.Conn
	kafkaConfig *kafka.KafkaConfig
}

func NewStreamClient(ctx context.Context, cfg *StreamClientConfig, kafkaConfig *kafka.KafkaConfig) StreamClientInterface {
	c := &StreamClient{
		ctx:         ctx,
		cfg:         cfg,
		tlsConfig:   &tls.Config{},
		readyFlag:   atomic.Bool{},
		kafkaConfig: kafkaConfig,
	}

	host, _, err := net.SplitHostPort(cfg.RealtimeStreamerUrl)
	if err != nil {
		// No port was specified, use the full server string
		host = cfg.RealtimeStreamerUrl
	}
	c.tlsConfig.ServerName = host
	return c
}

var (
	ErrConnectionIsNil = fmt.Errorf("connection is nil")
)

func (client *StreamClient) ConsumeRealtime(blockMsgsChan chan realtimeTypes.BlockInfo, txMsgsChan chan kafkaTypes.TransactionMessage, errorMsgsChan chan kafkaTypes.ErrorTriggerMessage, errorChan chan error) {
	defer client.Stop()

	// Send periodic heartbeats to keep connection alive
	go client.SendHeartbeat()

	for {
		select {
		case <-client.ctx.Done():
			log.Info("[Realtime] context done, stopping realtime loop")
			client.Stop()
			return
		default:
			if !client.readyFlag.Load() {
				if err := client.HandleRestart(); err != nil {
					log.Warn(fmt.Sprintf("[Realtime] failed to restart connection: %v", err))
					time.Sleep(client.cfg.RealtimeStreamerRetryDelay)
				}
				continue
			}

			message, err := client.readMessage()
			if err != nil {
				client.readyFlag.Store(false)
				continue
			}

			// Consume message
			switch message.TypeFlag {
			case streamer.UnknownTopicFlag:
				continue
			case streamer.ResetFlag:
				client.readyFlag.Store(false)
			case streamer.TopicBlock:
				var blockMsg realtimeTypes.BlockInfo
				if err := json.Unmarshal(message.Value, &blockMsg); err != nil {
					log.Warn(fmt.Sprintf("[Realtime] consume error, unmarshaling block message. error: %v", err))
					continue
				}
				select {
				case blockMsgsChan <- blockMsg:
				case <-client.ctx.Done():
					err := fmt.Errorf("context cancelled - stopping consume")
					errorChan <- err
					return
				}

			case streamer.TopicTx:
				var txMsg kafkaTypes.TransactionMessage
				if err := json.Unmarshal(message.Value, &txMsg); err != nil {
					log.Warn(fmt.Sprintf("[Realtime] consume error, unmarshaling transaction message. error: %v", err))
					continue
				}
				select {
				case txMsgsChan <- txMsg:
				case <-client.ctx.Done():
					err := fmt.Errorf("context cancelled - stopping consume")
					errorChan <- err
					return
				}

			case streamer.TopicError:
				var errorMsg kafkaTypes.ErrorTriggerMessage
				if err := json.Unmarshal(message.Value, &errorMsg); err != nil {
					log.Warn(fmt.Sprintf("[Realtime] consume error, unmarshaling error trigger message. error: %v", err))
					continue
				}
				select {
				case errorMsgsChan <- errorMsg:
				case <-client.ctx.Done():
					err := fmt.Errorf("context cancelled - stopping consume")
					errorChan <- err
					return
				}

			default:
				log.Warn(fmt.Sprintf("[Realtime] Unknown flag received: %s", message.TypeFlag))
				continue
			}
		}
	}
}

func (client *StreamClient) SendHeartbeat() {
	heartbeat := []byte{1}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-client.ctx.Done():
			return
		case <-ticker.C:
			err := client.tryWriteHeartbeat(heartbeat)
			if err != nil {
				log.Warn(fmt.Sprintf("[Realtime] Failed to send heartbeat: %v", err))
				client.readyFlag.Store(false)
			}
		}
	}
}

func (client *StreamClient) tryWriteHeartbeat(heartbeat []byte) error {
	client.connMutex.Lock()
	defer client.connMutex.Unlock()
	if client.conn == nil {
		return ErrConnectionIsNil
	}
	client.conn.SetWriteDeadline(time.Now().Add(client.cfg.RealtimeStreamerWriteTimeout))
	_, err := client.conn.Write(heartbeat)
	return err
}

func (client *StreamClient) readMessage() (*streamer.Message, error) {
	client.connMutex.Lock()
	defer client.connMutex.Unlock()

	if client.conn == nil {
		return nil, ErrConnectionIsNil
	}
	// Read 4-byte length prefix
	lengthBytes := make([]byte, 4)
	client.conn.SetReadDeadline(time.Now().Add(client.cfg.RealtimeStreamerReadTimeout))
	_, err := io.ReadFull(client.conn, lengthBytes)
	if err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lengthBytes)

	// Read the actual message
	messageBytes := make([]byte, length)
	_, err = io.ReadFull(client.conn, messageBytes)
	if err != nil {
		return nil, err
	}

	return &streamer.Message{
		TypeFlag: streamer.TypeFlag(messageBytes[0]),
		Value:    messageBytes[1:],
	}, nil
}

func (client *StreamClient) Start() error {
	client.connMutex.Lock()
	defer client.connMutex.Unlock()

	if client.conn != nil {
		return fmt.Errorf("[Realtime] streamclient start failed, conn not nil")
	}

	var err error
	// Open a TCP connection to the realtime stream server
	if client.cfg.RealtimeStreamerUseTLS {
		client.conn, err = tls.Dial("tcp", client.cfg.RealtimeStreamerUrl, client.tlsConfig)
	} else {
		client.conn, err = net.Dial("tcp", client.cfg.RealtimeStreamerUrl)
	}
	if err != nil {
		return fmt.Errorf("[Realtime] streamclient failed to connect to realtime streamserver %s: %w", client.cfg.RealtimeStreamerUrl, err)
	}
	client.conn.SetWriteDeadline(time.Now().Add(client.cfg.RealtimeStreamerWriteTimeout))
	client.readyFlag.Store(true)
	log.Info(fmt.Sprintf("[Realtime] Connected to realtime websocket server: %s", client.cfg.RealtimeStreamerUrl))
	return nil
}

func (client *StreamClient) Stop() error {
	client.connMutex.Lock()
	defer client.connMutex.Unlock()

	if client.conn != nil {
		err := client.conn.Close()
		client.conn = nil
		client.readyFlag.Store(false)
		return err
	}
	return nil
}

func (client *StreamClient) HandleRestart() error {
	if err := client.Stop(); err != nil {
		return err
	}
	return client.Start()
}
