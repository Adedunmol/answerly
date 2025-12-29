package queue

import (
	"context"
	"fmt"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"log"
	"os"
	"sync"
)

type Processor interface {
	Process() (*asynq.Task, error)
	ProcessorName() string
}

type Queue interface {
	Enqueue(processor Processor) error
}

type Client struct {
	client *asynq.Client
	once   sync.Once
}

func NewClient(ctx context.Context) (*Client, error) {
	var c Client

	redisUrl := os.Getenv("REDIS_URL")

	if redisUrl == "" {
		return nil, fmt.Errorf("REDIS_URL environment variable not set")
	}

	addr, err := redis.ParseURL(redisUrl)
	if err != nil {
		return &c, fmt.Errorf("error parsing redis url: %v", err)
	}

	c.once.Do(func() {
		log.Printf("setting up connection for asynq redis queue")
		c.client = asynq.NewClient(asynq.RedisClientOpt{Addr: addr.Addr, Password: "", DB: 0})
		log.Printf("connected to redis queue")
	})

	return &c, nil
}

func (c *Client) Enqueue(processor Processor) error {

	task, err := processor.Process()
	_, err = c.client.Enqueue(task)
	if err != nil {
		return fmt.Errorf("could not enqueue %s task for: %v", processor.ProcessorName(), err)
	}

	return nil
}

func (c *Client) GetClient() *asynq.Client {
	return c.client
}

func (c *Client) Close() error {
	log.Println("closing connection to asynq queue")
	return fmt.Errorf("error closing connection: %v", c.client.Close())
}

func (c *Client) Run(ctx context.Context) error {
	addr, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		return fmt.Errorf("error parsing redis url: %v", err)
	}

	queueServer := asynq.NewServer(asynq.RedisClientOpt{Addr: addr.Addr}, asynq.Config{})

	mux := asynq.NewServeMux()

	mux.HandleFunc(TypeEmailDelivery, HandleEmailTask)

	if err := queueServer.Run(mux); err != nil {
		return fmt.Errorf("error running queue server: %v", err)
	}
	return nil
}
