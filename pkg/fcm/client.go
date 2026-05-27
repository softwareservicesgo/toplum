package fcm

import (
	"context"
	"encoding/json"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"fmt"
	"google.golang.org/api/option"
)

type FCMClient struct {
	client *messaging.Client
}

func NewFCMClient(credentialsPath string) (*FCMClient, error) {
	opt := option.WithCredentialsFile(credentialsPath)

	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, fmt.Errorf("firebase init error: %w", err)
	}

	messagingClient, err := app.Messaging(context.Background())
	if err != nil {
		return nil, fmt.Errorf("messaging init error: %w", err)
	}

	return &FCMClient{client: messagingClient}, nil
}

func (f *FCMClient) SendToToken(ctx context.Context, token string, notification *NotificationDTO) error {
	jsonBytes, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	msg := &messaging.Message{
		Token: token,
		Data: map[string]string{
			"data": string(jsonBytes),
		},
	}
	_, err = f.client.Send(ctx, msg)
	return err
}

func (f *FCMClient) SendToTokens(ctx context.Context, tokens []string, notification *NotificationDTO) error {
	if len(tokens) == 0 {
		return fmt.Errorf("no tokens provided")
	}

	if len(tokens) > 500 {
		return fmt.Errorf("too many tokens provided")
	}

	jsonBytes, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Data: map[string]string{
			"data": string(jsonBytes),
		},
	}
	_, err = f.client.SendMulticast(ctx, msg)
	return err
}

const AllUsersTopicName = "all_users"

func (f *FCMClient) SendToTopic(ctx context.Context, topic string, notification *NotificationDTO) error {
	jsonBytes, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	msg := &messaging.Message{
		Topic: topic,
		Data: map[string]string{
			"data": string(jsonBytes),
		},
	}
	_, err = f.client.Send(ctx, msg)
	return err
}
