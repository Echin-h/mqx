package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	connect "mqx/core"
	"mqx/queue"
)

var producer *connect.Client

func InitProducer(ctx context.Context) {
	producer = connect.NewWithContext(ctx, queue.EndMqxQueue, queue.Url, func(conn *amqp.Connection) (*amqp.Channel, error) {
		ch, err := conn.Channel()
		_, err = ch.QueueDeclare(queue.EndMqxQueue,
			true,  // Durable
			false, // Delete when unused
			false, // Exclusive
			false, // No-wait
			nil,   // Arguments
		)
		if err != nil {
			panic(err.Error())
		}

		// 声明交换机
		err = ch.ExchangeDeclare(queue.EndMqxExchange,
			"x-delayed-message",
			true,
			false,
			false,
			false,
			amqp.Table{
				"x-delayed-type": "direct",
			},
		)
		if err != nil {
			panic(err.Error())
		}

		// 队列绑定（将队列、routing-key、交换机三者绑定到一起）
		err = ch.QueueBind(
			queue.EndMqxQueue,
			queue.EndPersonaliseRouterBindingKey,
			queue.EndMqxExchange, false, nil)

		return ch, err
	})
}

func SendMsg(data any, delay time.Duration) error {
	if producer == nil {
		return errors.New("producer is not init")
	}
	msg, _ := json.Marshal(&data)
	err := producer.Push(queue.EndMqxExchange,
		queue.EndPersonaliseRouterBindingKey, false, false, amqp.Publishing{
			ContentType:  "text/plain",
			DeliveryMode: amqp.Persistent,
			Body:         msg,
			Headers: map[string]interface{}{
				"x-delay": delay.Milliseconds(), // 消息从交换机过期时间,毫秒（x-dead-message插件提供）
			},
		})
	if err != nil {
		fmt.Println("send msg failed: ", err)
	}
	return err

}
