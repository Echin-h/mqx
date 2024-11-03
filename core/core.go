package connect

import (
	"context"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	ctx             context.Context
	queueName       string
	connection      *amqp.Connection
	channel         *amqp.Channel
	done            chan bool
	notifyConnClose chan *amqp.Error
	notifyChanClose chan *amqp.Error
	//notifyConfirm   chan amqp.Confirmation
	isReady    bool
	initchFunc func(ch *amqp.Connection) (*amqp.Channel, error)
}

const (
	// When reconnecting to the server after connection failure
	reconnectDelay = 5 * time.Second

	// When setting up the channel after a channel exception
	reInitDelay = 2 * time.Second

	// When resending messages the server didn't confirm
	resendDelay = 5 * time.Second
)

var (
	errNotConnected  = errors.New("not connected to a server")
	errAlreadyClosed = errors.New("already closed: not connected to the server")
	errShutdown      = errors.New("client is shutting down")
)

// New creates a new consumer state instance, and automatically
// attempts to connect to the server.
func newClient(ctx context.Context, queueName, addr string, initchFunc func(*amqp.Connection) (*amqp.Channel, error)) *Client {
	client := Client{
		ctx:        ctx,
		queueName:  queueName,
		done:       make(chan bool),
		initchFunc: initchFunc,
	}
	go client.handleReconnect(addr)
	return &client
}

func NewWithContext(ctx context.Context, queueName, addr string, initchFunc func(*amqp.Connection) (*amqp.Channel, error)) *Client {
	return newClient(ctx, queueName, addr, initchFunc)
}

// handleReconnect will wait for a connection error on
// notifyConnClose, and then continuously attempt to reconnect.
func (client *Client) handleReconnect(addr string) {
	for {
		client.isReady = false

		conn, err := client.connect(addr)

		if err != nil {
			fmt.Println(err)
			fmt.Println("Failed to connect rabbitmq. Retrying...")

			select {
			case <-client.done:
				return
			case <-time.After(reconnectDelay):
			}
			continue
		}

		if done := client.handleReInit(conn); done {
			break
		}
	}
}

// connect will create a new AMQP connection
func (client *Client) connect(addr string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(addr)

	if err != nil {
		return nil, err
	}

	client.changeConnection(conn)
	return conn, nil
}

// handleReconnect will wait for a channel error
// and then continuously attempt to re-initialize both channels
func (client *Client) handleReInit(conn *amqp.Connection) bool {
	for {
		client.isReady = false

		if client.initchFunc == nil {
			return false
		}
		ch, err := client.initchFunc(conn)
		if err != nil {
			fmt.Println("Failed to initialize channel. Retrying...", err)

			select {
			case <-client.done:
				return true
			case <-client.notifyConnClose:
				fmt.Println("Connection closed. Reconnecting...")
				return false
			case <-time.After(reInitDelay):
			}
			continue
		}

		client.changeChannel(ch)
		client.isReady = true
		fmt.Println("Connected to " + client.queueName)

		select {
		case <-client.done:
			return true
		case <-client.notifyConnClose:
			fmt.Println("Connection closed. Reconnecting...")
			return false
		case <-client.notifyChanClose:
			fmt.Println("Channel closed. Re-running init...")
		}
	}
}

// changeConnection takes a new connection to the queue,
// and updates the close listener to reflect this.
func (client *Client) changeConnection(connection *amqp.Connection) {
	client.connection = connection
	client.notifyConnClose = make(chan *amqp.Error, 1)
	client.connection.NotifyClose(client.notifyConnClose)
}

// changeChannel takes a new channel to the queue,
// and updates the channel listeners to reflect this.
func (client *Client) changeChannel(channel *amqp.Channel) {
	client.channel = channel
	client.notifyChanClose = make(chan *amqp.Error, 1)
	//client.notifyConfirm = make(chan amqp.Confirmation, 1)
	client.channel.NotifyClose(client.notifyChanClose)
	//client.channel.NotifyPublish(client.notifyConfirm)
}

// Push will push data onto the queue, and wait for a confirm.
// This will block until the server sends a confirm. Errors are
// only returned if the push action itself fails, see UnsafePush.
func (client *Client) Push(exchange string, key string, mandatory bool, immediate bool, msg amqp.Publishing) error {
	if !client.isReady {
		return errors.New("failed to push: not connected")
	}
	for {
		err := client.UnsafePush(exchange, key, mandatory, immediate, msg)
		if err != nil {
			fmt.Println("Push failed. Retrying...")
			select {
			case <-client.done:
				return errShutdown
			case <-time.After(resendDelay):
			}
			continue
		}
		fmt.Println("Msg Published: ", string(msg.Body))
		return nil
		//confirm := <-client.notifyConfirm
		//if confirm.Ack {
		//	logx.Debugf("Push confirmed [%d]!\n", confirm.DeliveryTag)
		//	return nil
		//}
	}
}

// UnsafePush will push to the queue without checking for
// confirmation. It returns an error if it fails to connect.
// No guarantees are provided for whether the server will
// receive the message.
func (client *Client) UnsafePush(exchange string, key string, mandatory bool, immediate bool, msg amqp.Publishing) error {
	if !client.isReady {
		return errNotConnected
	}

	ctx, cancel := context.WithTimeout(client.ctx, 30*time.Second)
	defer cancel()

	return client.channel.PublishWithContext(
		ctx,
		exchange,  // Exchange
		key,       // Routing key
		mandatory, // Mandatory
		immediate, // Immediate
		msg,
	)
}

// Consume will continuously put queue items on the channel.
// It is required to call delivery.Ack when it has been
// successfully processed, or delivery.Nack when it fails.
// Ignoring this will cause data to build up on the server.
func (client *Client) Consume(consumer string, autoAck bool, exclusive bool, noLocal bool, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	if !client.isReady {
		return nil, errNotConnected
	}

	if err := client.channel.Qos(
		1,     // prefetchCount
		0,     // prefrechSize
		false, // global
	); err != nil {
		return nil, err
	}

	return client.channel.Consume(
		client.queueName,
		consumer,  // Consumer
		autoAck,   // Auto-Ack
		exclusive, // Exclusive
		noLocal,   // No-local
		noWait,    // No-Wait
		args,      // Args
	)
}

func (client *Client) NotifyClose(c chan *amqp.Error) {
	client.channel.NotifyClose(c)
}

// Close will cleanly shut down the channel and connection.
func (client *Client) Close() error {
	if !client.isReady {
		return errAlreadyClosed
	}
	close(client.done)
	err := client.channel.Close()
	if err != nil {
		return err
	}
	err = client.connection.Close()
	if err != nil {
		return err
	}

	client.isReady = false
	return nil
}
