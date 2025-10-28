// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

const (
	publisher mode = iota
	subscriber
)

const (
	defUsername    = ""
	defToken       = ""
	defURL         = "localhost:8080/mqtt"
	defScheme      = "ws"
	defRetry       = 0
	defMode        = publisher
	defChannel     = ""
	defPubPoolSize = 1
)

type mode int

type session struct {
	conn    *conn
	mode    mode
	channel string
	qos     byte
}

type conn struct {
	ID       string
	client   mqtt.Client
	username string
	token    string
	url      string
	scheme   string
	retry    int
}

type config struct {
	url      string
	scheme   string
	username string
	token    string
	mode     mode
	retry    int
}

func (c conn) New(cfg config) (*conn, error) {
	cid, err := c.id()
	if err != nil {
		return nil, err
	}

	newConn := &conn{
		ID:       cid,
		username: cfg.username,
		token:    cfg.token,
		url:      cfg.url,
		scheme:   cfg.scheme,
		retry:    cfg.retry,
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("%s://%s", newConn.scheme, newConn.url))
	opts.SetClientID(newConn.ID)
	opts.SetUsername(newConn.username)
	opts.SetPassword(newConn.token)

	mqttClient := mqtt.NewClient(opts)

	// Try to connect with retry
	for i := 0; i <= c.retry; i++ {
		if token := mqttClient.Connect(); token.Wait() && token.Error() == nil {
			break
		} else {
			fmt.Println(token.Error())
			time.Sleep(1 * time.Second)
		}
	}

	newConn.client = mqttClient

	return newConn, nil
}

func (c conn) id() (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func (cfg config) load() (config, error) {
	c := config{
		url:      defURL,
		username: defUsername,
		token:    defToken,
		scheme:   defScheme,
		retry:    defRetry,
		mode:     defMode,
	}
	return c, nil
}

func main() {
	// Connect single client over WS and publish every 10 sec
	cfg, err := config{}.load()
	if err != nil {
		fmt.Println("Error loading config", err.Error())
	}
	sconn, err := conn{}.New(cfg)
	if err != nil {
		fmt.Println("Error making New subscriber connection", err.Error())
	}
	defer sconn.client.Disconnect(200)
	// Make 2 sessions sub and pub which will publish rand msg in time interval
	sub := session{
		conn:    sconn,
		mode:    subscriber,
		channel: defChannel,
		qos:     2,
	}

	var mCallback mqtt.MessageHandler = func(_ mqtt.Client, msg mqtt.Message) {
		fmt.Printf("Receiving message: %s\n", msg.Payload())
	}
	if token := sub.conn.client.Subscribe(sub.channel, sub.qos, mCallback); token.Wait() && token.Error() != nil {
		fmt.Println("Error subscribing", token.Error())
		sconn.client.Disconnect(100)
	}

	pconn, err := conn{}.New(cfg)
	if err != nil {
		fmt.Println("Error making New publisher connection", err.Error())
	}
	defer pconn.client.Disconnect(200)

	errc := make(chan error, 2)

	// Create publishers herd
	for i := 0; i < defPubPoolSize; i++ {
		pconn, err := conn{}.New(cfg)
		if err != nil {
			fmt.Println("Error making New publisher connection", err.Error())
		}
		defer pconn.client.Disconnect(200)
		pub := session{
			conn:    pconn,
			mode:    publisher,
			channel: defChannel,
			qos:     2,
		}
		go pub.random(10*time.Second, errc)
	}
	fmt.Println("Connected! Waiting for publisher to start...")
	fmt.Println("Error during publish: ", <-errc)
	<-errc
}

func (s session) random(d time.Duration, errc chan<- error) {
	msg := "Ping from client " + s.conn.ID
	for x := range time.Tick(d) {
		fmt.Println("Publishing ", x)
		if token := s.conn.client.Publish(s.channel, s.qos, false, msg); token.Wait() && token.Error() != nil {
			errc <- token.Error()
			break
		}
	}
}
