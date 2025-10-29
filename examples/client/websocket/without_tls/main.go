// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/absmach/mgate/examples/client/websocket"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	brokerAddress = "ws://localhost:8083/mgate-ws"
	topic         = "test/topic"
	payload       = "Hello mGate"
)

func main() {
	// Replace these with your MQTT broker details
	fmt.Printf("Subscribing to topic %s without TLS\n", topic)
	subClient, err := websocket.Connect(brokerAddress, nil)
	if err != nil {
		panic(err)
	}
	defer subClient.Disconnect(250)

	done := make(chan struct{}, 1)
	if token := subClient.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) { onMessage(c, m, done) }); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	fmt.Printf("Publishing to topic %s without TLS\n", topic)
	pubClient, err := websocket.Connect(brokerAddress, nil)
	if err != nil {
		panic(err)
	}

	defer pubClient.Disconnect(250)

	pubClient.Publish(topic, 0, false, payload)
	<-done

	invalidPathBrokerAddress := brokerAddress + "/invalid_path"
	fmt.Printf("Publishing to topic %s without TLS to invalid path %s \n", topic, invalidPathBrokerAddress)
	pubClientInvalidPath, err := websocket.Connect(invalidPathBrokerAddress, nil)
	if err == nil {
		pubClientInvalidPath.Disconnect(250)
		panic("some thing went wrong")
	}
	fmt.Printf("Failed to connect with invalid path %s,error : %s\n", invalidPathBrokerAddress, err.Error())
}

func onMessage(_ mqtt.Client, m mqtt.Message, done chan struct{}) {
	fmt.Printf("Subscription Message Received, Topic : %s, Payload %s\n", m.Topic(), string(m.Payload()))
	done <- struct{}{}
}
