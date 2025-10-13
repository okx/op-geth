package streamer

type TypeFlag int

const (
	ResetFlag        TypeFlag = iota // 0
	UnknownTopicFlag                 // 1
	TopicBlock                       // 2
	TopicTx                          // 3
	TopicError                       // 4
)

type Message struct {
	TypeFlag TypeFlag
	Value    []byte
}
