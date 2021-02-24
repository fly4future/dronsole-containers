package main

import (
	"context"
	"log"

	"cloud.google.com/go/pubsub"
)

func pullIoTCoreMessages(subscription string) {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create pubsub client %v", err)
	}
	defer client.Close()

	sub := client.Subscription(subscription)

	messages := make(chan *pubsub.Message)
	defer close(messages)

	go func() {
		for msg := range messages {
			deviceID, ok := msg.Attributes["deviceId"]
			if !ok {
				log.Printf("Pubsub message '%s' doesn't contain attribute deviceId", msg.ID)
				msg.Ack()
				return
			}
			topic, ok := msg.Attributes["subFolder"]
			if !ok {
				log.Printf("Pubsub message '%s' doesn't contain attribute subFolder", msg.ID)
				msg.Ack()
				return
			}

			handleMQTTEvent(deviceID, topic, msg.Data)
			msg.Ack()
		}
	}()

	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		messages <- msg
	})
	if err != nil {
		log.Printf("Failed to receive pubsub messages: %v", err)
	}
}
