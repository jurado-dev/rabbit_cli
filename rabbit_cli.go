package rabbit_cli

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"log"
	"os/exec"
)

type ExchangeConfig struct {
	Name       string
	Kind       string
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp.Table
}

func NewExchangeConfig(name string, kind string, durable bool) ExchangeConfig {
	return ExchangeConfig{Name: name, Kind: kind, Durable: durable}
}

type QueueConfig struct {
	Name           string
	Durable        bool
	AutoDelete     bool
	NoWait         bool
	Exclusive      bool
	Args           amqp.Table
	RouteKey       string
	PrefetchCount  int
	PrefetchSize   int
	PrefetchGlobal bool
}

func NewQueueConfig(name string, durable, exclusive bool) QueueConfig {
	return QueueConfig{Name: name, Durable: durable, Exclusive: exclusive}
}

type ConsumerConfig struct {
	Handler   func(messages <-chan amqp.Delivery, done chan bool)
	Consumer  string
	AutoAck   bool
	Exclusive bool
	NoLocal   bool
	NoWait    bool
	Args      amqp.Table
}

func NewConsumerConfig(handler func(messages <-chan amqp.Delivery, done chan bool), autoAck bool) ConsumerConfig {
	c := ConsumerConfig{}
	c.AutoAck = autoAck
	c.Handler = handler
	return c
}

type MessageConfig struct {
	Body          []byte
	RouteKey      string
	Mandatory     bool
	Immediate     bool
	CorrelationId string
	ReplyQueue    string
	ContentType   string
}

func NewMessageConfig(data interface{}, routeKey string) (MessageConfig, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return MessageConfig{}, err
	}

	corId, err := NewCorrelationId()
	if err != nil {
		return MessageConfig{}, err
	}

	return MessageConfig{Body: body, RouteKey: routeKey, ContentType: "text/plain", CorrelationId:corId}, nil
}

//	NewCorrelationId generates a unique id using the linux tool uuidgen
func NewCorrelationId() (string, error) {
	uid, err := exec.Command("uuidgen").Output()
	if err != nil {
		return "", err
	}
	return string(uid), nil
}

type RabbitCli struct {
	url string
}

func NewRabbitCli(url string) *RabbitCli {
	return &RabbitCli{url: url}
}

func (rc *RabbitCli) newConnection() (*amqp.Connection, error) {

	con, err := amqp.Dial(rc.url)
	if err != nil {
		return nil, err
	}

	return con, nil
}

// Publish connects to a rabbit server and publish a message to an exchange (if exchange name is not defined the msg will be
// published to the default exchange). If you need to publish a RPC response, the reply queue name must be passed in the
// field RouteKey of the message structure.
func (rc *RabbitCli) Publish(exc ExchangeConfig, msg MessageConfig) error {

	con, err := rc.newConnection()
	if err != nil {
		return err
	}
	defer con.Close()

	channel, err := con.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	if exc.Name != "" {
		log.Printf("Creating the exchange %s", exc.Name)
		err = channel.ExchangeDeclare(exc.Name, exc.Kind, exc.Durable, exc.AutoDelete, exc.Internal, exc.NoWait, exc.Args)
		if err != nil {
			return err
		}
	}

	return channel.Publish(exc.Name, msg.RouteKey, msg.Mandatory, msg.Immediate,
		amqp.Publishing{
			CorrelationId: msg.CorrelationId,
			ReplyTo:       msg.ReplyQueue,
			ContentType:   msg.ContentType,
			Body:          msg.Body,
		},
	)
}

//	Consume obtains the messages from a queue that is bound to some exchange. Then delegates the manage of the messages
//	to the message handler implementation
func (rc *RabbitCli) Consume(exc ExchangeConfig, queue QueueConfig, consumer ConsumerConfig) error {

	con, err := rc.newConnection()
	if err != nil {
		return err
	}
	defer con.Close()

	channel, err := con.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	if err = channel.Qos(queue.PrefetchCount, queue.PrefetchSize, queue.PrefetchGlobal); err != nil {
		return err
	}

	_, err = channel.QueueDeclare(queue.Name, queue.Durable, queue.AutoDelete, queue.Exclusive, queue.NoWait, queue.Args)
	if err != nil {
		return err
	}

	if exc.Name != "" {

		log.Printf("Binding the queue to the exchange '%s'", exc.Name)

		err = channel.ExchangeDeclare(exc.Name, exc.Kind, exc.Durable, exc.AutoDelete, exc.Internal, exc.NoWait, exc.Args)
		if err != nil {
			return err
		}

		if err = channel.QueueBind(queue.Name, queue.RouteKey, exc.Name, queue.NoWait, queue.Args); err != nil {
			return err
		}
	}

	messages, err := channel.Consume(queue.Name, consumer.Consumer, consumer.AutoAck, consumer.Exclusive, consumer.NoLocal, consumer.NoWait, consumer.Args)
	if err != nil {
		return err
	}

	done := make(chan bool)
	go consumer.Handler(messages, done)
	<-done

	return nil
}

//	MakeRpc publishes a rpc message and handles the response. The message must contain a correlationId
func (rc *RabbitCli) MakeRpc(exc ExchangeConfig, msg MessageConfig) ([]byte, error) {

	//	Connecting to rabbit server to create a unique queue
	con, err := rc.newConnection()
	if err != nil {
		return nil, err
	}
	defer con.Close()

	channel, err := con.Channel()
	if err != nil {
		return nil, err
	}
	defer channel.Close()

	//	Creating a new unique queue
	replyQueue, err := channel.QueueDeclare("", false, false, true, false, nil)
	if err != nil {
		return nil, err
	}

	//	Handling the response messages from the unique queue
	responses, err := channel.Consume(replyQueue.Name, "", true, true, false, false, nil)
	if err != nil {
		return nil, err
	}

	//	Adding to msg the name of the unique ID
	msg.ReplyQueue = replyQueue.Name

	//	Publish the message to the exchange
	err = rc.Publish(exc, msg)
	if err != nil {
		return nil, err
	}

	//	Waiting and handling responses
	for response := range responses {

		//	Since the queue name is unique no other process must send messages tho that queue. Therefore if the IDS are
		//	different, just returns a error. Is not need to wait other responses
		if response.CorrelationId != msg.CorrelationId {
			response.Reject(false)
			return nil, errors.New("the correlation ids do not match")
		}

		return response.Body, nil
	}

	return nil, errors.New("response was not obtained")
}
