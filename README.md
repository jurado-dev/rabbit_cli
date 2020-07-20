# Rabbit-Cli

Rabbit-cli is a light client to execute basic RabbitMQ (AMQP) operations

## Installation

```bash
go get github.com/jurado-dev/rabbit_cli 
```

## Usage

### Publish
```golang
package main

import (
	"github.com/jurado-dev/rabbit_cli"
    "log"

)

func main() {

	exc := rabbit_cli.NewExchangeConfig("test", "fanout", true)

	data := make(map[string]string)
    data["foo"] = "foo"


	msg, err := rabbit_cli.NewMessageConfig(data, "any")
	if err != nil {
                log.Printf("can not set message configuration - %s", err.Error())
		return
	}

    rabbitUrl := "amqp://rabbitmq:rabbitmq@localhost:5672/"

	err = rabbit_cli.NewRabbitCli(rabbitUrl).Publish(exc, msg)
	if err != nil {
		log.Printf("can not publish the message - %s", err.Error())
		return
	}
  
     log.Printf("the message was published")
}
```

### Consume

``` go
import (
	"github.com/jurado-dev/rabbit_cli"
	"github.com/streadway/amqp"
	"log"
)

//	Consume contains the logic used to process a message
func Consume(messages <- chan amqp.Delivery, done chan bool) {

	for msg := range messages {
		log.Printf("received a new message: %s", string(msg.Body))
		msg.Ack(false)
		log.Printf("done with message")
	}

	done <- true
}

func main () {
	
	exc := rabbit_cli.NewExchangeConfig("test", "fanout", true)
	
	queue := rabbit_cli.NewQueueConfig("test_queue", true, false)
	queue.PrefetchCount = 1
	
	consumer := rabbit_cli.NewConsumerConfig(Consume, false)

	rabbitUrl := "amqp://rabbitmq:rabbitmq@localhost:5672/"
	err := rabbit_cli.NewRabbitCli(rabbitUrl).Consume(exc, queue, consumer)
	if err != nil {
		log.Printf("can not consume messages - %s", err.Error())
		return
	}
}
```

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License
[MIT](https://choosealicense.com/licenses/mit/)
