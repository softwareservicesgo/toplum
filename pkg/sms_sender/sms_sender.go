package sms_sender

import (
	"errors"
	"fmt"
	"sync"

	"restaurants/pkg/sms_sender/socket_helper"
)

type Config struct {
	BasicOptMsg string `yaml:"BasicOptMsg"`
}

type Client struct {
	basicOptMsg string
	client      *socket_helper.Client
	mu          sync.RWMutex
}

func NewSmsSender(cfg Config) (*Client, error) {
	return &Client{
		basicOptMsg: cfg.BasicOptMsg,
	}, nil
}

func (c *Client) RegisterClient(client *socket_helper.Client) {
	c.mu.Lock()
	c.client = client
	c.mu.Unlock()
}

func (c *Client) SendOtp(phone string, otp int) error {
	msg := fmt.Sprintf(c.basicOptMsg, otp)

	return c.sendSms(phone, msg)
}

func (c *Client) sendSms(phone, message string) error {
	msg := socket_helper.Message{
		PhoneNumber: phone,
		Message:     message,
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.client == nil || c.client.IsClosed.Load() {
		return errors.New("socket client is closed")
	}

	c.client.Write(msg)

	return nil
}
