package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	connect "mqx/core"
	"mqx/queue"
)

var consumer *connect.Client

func InitConsumer(ctx context.Context) {
	consumer = connect.NewWithContext(ctx, queue.EndMqxQueue, queue.Url, func(conn *amqp.Connection) (*amqp.Channel, error) {
		return conn.Channel()
	})
	go consume()
}

func consume() {
retry:
	<-time.After(2 * time.Second)
	if consumer == nil {
		fmt.Println("Empty consumer")
		goto retry
	}

	deliveries, err := consumer.Consume(queue.EndPersonaliseRouterBindingKey, false, false, false, false, nil)
	if err != nil {
		fmt.Println("Could not start consuming: ", err)
		goto retry
	}

	// This channel will receive a notification when a channel closed event
	// happens. This must be different from Client.notifyChanClose because the
	// library sends only one notification and Client.notifyChanClose already has
	// a receiver in handleReconnect().
	// Recommended to make it buffered to avoid deadlocks
	chClosedCh := make(chan *amqp.Error, 1)
	consumer.NotifyClose(chClosedCh)

	for {
		select {
		case amqErr := <-chClosedCh:
			// This case handles the event of closed channel e.g. abnormal shutdown
			fmt.Println("AMQP Channel closed due to: ", amqErr)
			goto retry

		case delivery := <-deliveries:
			var req queue.Message
			if _err := json.Unmarshal(delivery.Body, &req); _err != nil {
				_ = delivery.Reject(false)
				fmt.Println("Unmarshal error: ", _err, " body: ", string(delivery.Body))
				break
			}
			fmt.Println("Receive message: ", req)

			// Process the message here
			if !ProcessMsg(req) {
				_ = delivery.Reject(false)
				break
			}

			if err := delivery.Ack(false); err != nil {
				fmt.Println("Error acknowledging message: ", err)
			}
		}
	}
}
