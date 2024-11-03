package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"mqx/queue"
	"mqx/queue/consumer"
	"mqx/queue/producer"
)

func initMqx(ctx context.Context) {
	producer.InitProducer(ctx)
	consumer.InitConsumer(ctx)
	fmt.Println("MQX init success")
}

func main() {
	ctx := context.Background()
	initMqx(ctx)

	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		delay := time.Second * 10
		err := producer.SendMsg(queue.Message{
			FuncName: queue.MsgHelloWorld,
			Data: map[string]any{
				"UserId": "123",
				"KbId":   "456",
			},
		}, delay)
		if err != nil {
			fmt.Println(err)
			return
		}
		_, err = w.Write([]byte("hello world"))
		if err != nil {
			fmt.Println("write response failed: ", err)
			return
		}
	})

	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("server start failed: ", err)
		return
	}

	fmt.Println("hello world success")

}
