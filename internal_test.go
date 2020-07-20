package rabbit_cli

import (
	"github.com/streadway/amqp"
	"log"
	"testing"
)

//	To test, first you must execute the consumer test in order to create the queue that will receive the exchange messages.

//	Modify this url with your rabbitMQ configuration
const rabbitUrl = "amqp://rabbitmq:rabbitmq@localhost:5672/"

//	Handles the messages from the defined queue
func Handle(messages <-chan amqp.Delivery, done chan bool) {

	//	Iterating the messages channel
	for msg := range messages {

		log.Printf("Received a new message: %s", string(msg.Body))
		log.Printf("The message route key is: %s", msg.RoutingKey)

		//	Responding if the msg is RPC
		if msg.ReplyTo != "" {

			log.Printf("Responding to RPC message")

			//	Creating a fake RPC response
			result := make(map[string]bool)
			result["success"] = true

			//	Creating the new message configuration
			msgConfig, err := NewMessageConfig(result, "")
			if err != nil {
				log.Printf("can not create the RPC response - %s", err.Error())
				msg.Reject(true)
				continue
			}

			//	Adding to message the RPC credentials
			msgConfig.RouteKey = msg.ReplyTo // When responding, the queu name must be set at the route key field
			msgConfig.CorrelationId = msg.CorrelationId

			//	Publishing the RPC response
			err = NewRabbitCli(rabbitUrl).Publish(ExchangeConfig{}, msgConfig)
			if err != nil {
				log.Printf("can not publish the RPC response - %s", err.Error())
				msg.Reject(true)
				continue
			}

			log.Printf("The RPC was responded to the queue: %s", msg.ReplyTo)
			msg.Ack(false)
			continue
		}


		msg.Ack(false)
	}

	done <- true
}

func TestPublish(t *testing.T) {

	//	Setting the exchange configuration
	exc := NewExchangeConfig("testing", "topic", true)

	//	Creating a fake message
	data := make(map[string]string)
	data["testing"] = "message"

	//	Setting the message configuration
	msg, err := NewMessageConfig(data, "route.other")
	if err != nil {
		t.Errorf("can not create the message config - %s", err.Error())
		return
	}

	//	Publishing the msg to the exchange
	err = NewRabbitCli(rabbitUrl).Publish(exc, msg)
	if err != nil {
		t.Errorf("error while publishing a message - %s", err.Error())
		return
	}

	t.Log("the message was published!")
}

func TestConsume(t *testing.T) {

	//	Setting the exchange configuration
	exc := NewExchangeConfig("testing", "topic", true)

	//	Setting the queue configuration
	queue := NewQueueConfig("testing", true, false)
	queue.RouteKey = "#" //	Accepts any key
	queue.PrefetchCount = 1 //	Receives one per one

	//	Setting consumer configuration (passing the message handler)
	consumer := NewConsumerConfig(Handle, false)

	//	Consuming the messages from the queue
	err := NewRabbitCli(rabbitUrl).Consume(exc, queue, consumer)
	if err != nil {
		t.Errorf("can not consume messages - %s", err.Error())
		return
	}

	t.Log("All messages were consumed - maybe this will never researched")
}

func TestRpc(t *testing.T) {

	//	Defining the destination exchange
	exc := NewExchangeConfig("testing", "topic", true)

	//	Creating a fake message
	data := make(map[string]bool)
	data["send test"] = true

	//	Setting the message configuration
	msg, err := NewMessageConfig(data, "example.route")
	if err != nil {
		t.Errorf("can not create the message - %s", err.Error())
		return
	}

	//	Executing the RPC request
	result, err := NewRabbitCli(rabbitUrl).MakeRpc(exc, msg)
	if err != nil {
		t.Errorf("error while executing the RPC request - %s", err.Error())
		return
	}

	t.Logf("obtained rpc response - %s", string(result))
}