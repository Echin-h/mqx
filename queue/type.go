package queue

const (
	Url                            = "amqp://root:123456@localhost:5672/"
	EndMqxQueue                    = "mqx-queue-endmqx"
	EndMqxExchange                 = "mqx-exchange-endmqx"
	EndPersonaliseRouterBindingKey = "/mqx/endmqx"

	MsgHelloWorld = "hello world"
)

type Message struct {
	FuncName string
	Data     map[string]any
}
