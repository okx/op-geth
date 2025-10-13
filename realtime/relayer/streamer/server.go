package streamer

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/realtime/kafka"
	kafkaTypes "github.com/ethereum/go-ethereum/realtime/kafka/types"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

const (
	MaxWSConnections = 100 // Maximum number of connected clients
	TimeoutInterval  = 2 * time.Second
)

// Streamserver commands
const (
	StartCommand = 1 << iota
	StopCommand
)

var (
	MaxMessageChanSize      = 10_000
	MaxMessageCacheSize     = 100
	MinRealtimeLoopWaitTime = 10 * time.Millisecond

	errorFlag = atomic.Bool{}
	resetFlag = atomic.Bool{}
)

type StreamServerInterface interface {
	Start() error
	Stop() error
	BroadcastMessage(message []byte)
	GetConnectionsCount() int
	getResetFlagMessage() []byte
	fromKafkaMessage(kafkaMessage *sarama.ConsumerMessage) []byte
}

type StreamServer struct {
	readyFlag      atomic.Bool
	ln             net.Listener
	clients        map[string]*WsClient
	mutexLock      sync.RWMutex
	streamerConfig *StreamerConfig
	kafkaConfig    *kafka.KafkaConfig
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewServer(streamerConfig *StreamerConfig, kafkaConfig *kafka.KafkaConfig) StreamServerInterface {
	ctx, cancel := context.WithCancel(context.Background())
	return &StreamServer{
		readyFlag:      atomic.Bool{},
		clients:        make(map[string]*WsClient),
		streamerConfig: streamerConfig,
		kafkaConfig:    kafkaConfig,
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (server *StreamServer) Start() error {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(int(server.streamerConfig.Port)))
	if err != nil {
		log.Error(fmt.Sprintf("Error creating streamserver %d: %v", server.streamerConfig.Port, err))
		return err
	}
	server.ln = ln

	// Handle relaying realtime data to active ws connections
	go server.relayRealtime()

	// Check ws connections inactivity timeout
	go server.checkHeartbeats()

	// Handle new ws connections
	go server.waitConnections()

	return nil
}

func (server *StreamServer) Stop() error {
	// Signal shutdown
	server.readyFlag.Store(false)
	server.cancel()

	// Close listener
	if server.ln != nil {
		server.ln.Close()
	}

	// Close all client connections
	server.mutexLock.Lock()
	for id, client := range server.clients {
		if client != nil && client.conn != nil {
			if err := client.conn.Close(); err != nil {
				log.Warn(fmt.Sprintf("Error closing client %s: %v", id, err))
			}
		}
	}
	server.clients = make(map[string]*WsClient)
	server.mutexLock.Unlock()

	log.Info("Stream server stopped gracefully")
	return nil
}

func (server *StreamServer) BroadcastMessage(fullMsg []byte) {
	server.mutexLock.RLock()
	defer server.mutexLock.RUnlock()

	dropped := 0
	for _, client := range server.clients {
		if !client.SendMessage(fullMsg) {
			dropped++
			log.Warn(fmt.Sprintf("Dropped message for client %s (queue full)", client.id))
		}
	}

	if dropped > 0 {
		log.Warn(fmt.Sprintf("Dropped messages for %d clients due to full queues", dropped))
	}
}

func (server *StreamServer) GetConnectionsCount() int {
	return server.getConnectionsCount()
}

func (server *StreamServer) relayRealtime() {
	defer server.cancel()
	rawMsgsChan := make(chan *sarama.ConsumerMessage, MaxMessageChanSize) // Raw messages with topic info
	errorChan := make(chan error, 1)

	// Consumer that returns raw bytes
	kafkaConsumer, err := kafka.NewKafkaConsumer(*server.kafkaConfig, true)
	if err != nil {
		log.Warn(fmt.Sprintf("[Realtime] Failed to initialize kafka consumer: %v", err))
		return
	}

	go kafkaConsumer.ConsumeRawKafka(server.ctx, rawMsgsChan, errorChan)

	for {
		select {
		case <-server.ctx.Done():
			log.Info("Server shutdown requested, notifying clients")
			bytes := server.getResetFlagMessage()
			server.BroadcastMessage(bytes)
			return

		case rawMsg := <-rawMsgsChan:
			bytes := server.fromKafkaMessage(rawMsg)
			server.BroadcastMessage(bytes)

		case err := <-errorChan:
			log.Error(fmt.Sprintf("Kafka consumer failed: %v", err))
			bytes := server.getResetFlagMessage()
			server.BroadcastMessage(bytes)
		}
	}
}

func (server *StreamServer) checkHeartbeats() {
	for {
		select {
		case <-server.ctx.Done():
			return
		default:
			time.Sleep(server.streamerConfig.HeartbeatCheckInterval)
			now := time.Now()

			var clientsToKill = map[string]struct{}{}
			server.mutexLock.Lock()
			for _, client := range server.clients {
				hb := time.Unix(0, client.heartbeat.Load())
				if hb.Add(server.streamerConfig.InactivityTimeout).Before(now) {
					clientsToKill[client.id] = struct{}{}
				}
			}
			server.mutexLock.Unlock()

			for clientID := range clientsToKill {
				log.Warn(fmt.Sprintf("Closing inactive client %s", clientID))
				server.closeConnection(clientID)
			}
		}
	}
}

func (server *StreamServer) waitConnections() {
	defer server.ln.Close()
	for {
		select {
		case <-server.ctx.Done():
			return
		default:
			conn, err := server.ln.Accept()
			if err != nil {
				if !server.readyFlag.Load() {
					return
				}

				// Handle timeout (expected)
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}

				// Handle closed connection
				if errors.Is(err, net.ErrClosed) {
					log.Info("Listener closed, stopping connection acceptor")
					return
				}

				// Other errors
				log.Error(fmt.Sprintf("Error accepting new ws connection: %v", err))
				time.Sleep(TimeoutInterval)
				continue
			}
			// Check max connections limit
			if server.getConnectionsCount() >= MaxWSConnections {
				log.Warn(fmt.Sprintf("Max connections limit reached, closing connection: %v", conn.RemoteAddr()))
				conn.Close()
				time.Sleep(TimeoutInterval)
				continue
			}
			go server.handleConnection(conn)
		}
	}
}

func (server *StreamServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	id := conn.RemoteAddr().String()
	log.Info(fmt.Sprintf("New ws connection: %s", id))

	server.mutexLock.Lock()
	client := NewWsClient(conn, conn.RemoteAddr().String())
	server.clients[id] = client
	server.mutexLock.Unlock()

	go server.clientMessageWorker(client)

	for {
		err := ReadHeartbeat(client)
		if err != nil {
			server.closeConnection(id)
			return
		}
	}
}

func (server *StreamServer) clientMessageWorker(client *WsClient) {
	defer func() {
		log.Info(fmt.Sprintf("Message worker stopped for client %s", client.id))
	}()

	for {
		select {
		case <-client.ctx.Done():
			// Client is being closed
			return

		case msg, ok := <-client.msgQueue:
			if !ok {
				return
			}

			_, err := TimeoutWrite(client, msg, server.streamerConfig.WriteTimeout)
			if err != nil {
				log.Error(fmt.Sprintf("Error sending message to client %s, error: %v", client.id, err))
				server.closeConnection(client.id)
				return
			}
		}
	}
}

func (server *StreamServer) closeConnection(id string) {
	server.mutexLock.Lock()
	defer server.mutexLock.Unlock()

	client := server.clients[id]
	if client != nil && client.status != Killed {
		client.Close()
		client.status = Killed
		if client.conn != nil {
			client.conn.Close()
		}
		delete(server.clients, id)
	}
}

func (server *StreamServer) getConnectionsCount() int {
	server.mutexLock.RLock()
	defer server.mutexLock.RUnlock()
	return len(server.clients)
}

func (server *StreamServer) typeFlagFromTopic(topic string) TypeFlag {
	switch topic {
	case server.kafkaConfig.BlockTopic:
		return TopicBlock
	case server.kafkaConfig.TxTopic:
		return TopicTx
	case server.kafkaConfig.ErrorTopic:
		return TopicError
	default:
		return UnknownTopicFlag
	}
}

func (server *StreamServer) getResetFlagMessage() []byte {
	return serializeMessage(ResetFlag, nil)
}

func (server *StreamServer) fromKafkaMessage(kafkaMessage *sarama.ConsumerMessage) []byte {
	typeFlag := server.typeFlagFromTopic(kafkaMessage.Topic)

	if typeFlag == TopicBlock {
		var blockMsg realtimeTypes.BlockInfo
		if err := json.Unmarshal(kafkaMessage.Value, &blockMsg); err != nil {
			log.Warn(fmt.Sprintf("[Realtime] consume error, unmarshaling block message. error: %v", err))
		} else {
			// Overwrite blockTime with current time
			blockMsg.Header.Time = uint64(time.Now().Unix())
			// Re-marshal the updated block message
			if updatedValue, err := json.Marshal(blockMsg); err != nil {
				log.Warn(fmt.Sprintf("[Realtime] consume error, marshaling updated block message. error: %v", err))
			} else {
				kafkaMessage.Value = updatedValue
			}
		}
	} else if typeFlag == TopicTx {
		var txMsg kafkaTypes.TransactionMessage
		if err := json.Unmarshal(kafkaMessage.Value, &txMsg); err != nil {
			log.Warn(fmt.Sprintf("[Realtime] consume error, unmarshaling transaction message. error: %v", err))
		} else {
			// Overwrite blockTime with current time
			txMsg.BlockTime = uint64(time.Now().Unix())
			// Re-marshal the updated block message
			if updatedValue, err := json.Marshal(txMsg); err != nil {
				log.Warn(fmt.Sprintf("[Realtime] consume error, marshaling updated block message. error: %v", err))
			} else {
				kafkaMessage.Value = updatedValue
			}
		}
	}

	value := kafkaMessage.Value

	return serializeMessage(typeFlag, value)
}

func serializeMessage(typeFlag TypeFlag, value []byte) []byte {
	lengthPrefix := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthPrefix, uint32(len(value))+1)
	return append(append(lengthPrefix, byte(typeFlag)), value...)
}
