package streamer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

func ReadHeartbeat(client *WsClient) error {
	buffer := make([]byte, 1)
	_, err := io.ReadFull(client.conn, buffer)
	if err != nil {
		if err == io.EOF {
			log.Debug(fmt.Sprintf("Client %s close connection", client.conn.RemoteAddr().String()))
		} else {
			log.Warn(fmt.Sprintf("Error reading from client %s, error: %v", client.conn.RemoteAddr().String(), err))
		}
		return err
	}
	client.SetHeartbeat()
	return nil
}

// TimeoutWrite sets a deadline time before write
func TimeoutWrite(client *WsClient, data []byte, timeout time.Duration) (int, error) {
	err := client.conn.SetWriteDeadline(time.Now().Add(timeout))
	if err != nil {
		log.Warn(fmt.Sprintf("Error setting write deadline: %v", err))
	}
	n, err := client.conn.Write(data)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			log.Debug(fmt.Sprintf("Write deadline exceeded for client %s, error: %v", client.id, err))
		}
	} else {
		client.SetHeartbeat()
	}

	return n, err
}
