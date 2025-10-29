// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/absmach/mgate/examples/client/websocket"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	brokerAddress = "wss://localhost:8084/mgate-ws"
	topic         = "test/topic"
	payload       = "Hello mGate"
	certFile      = ""
	keyFile       = ""
	serverCAFile  = "ssl/certs/ca.crt"
	clientCAFile  = ""
)

func main() {
	// Replace these with your MQTT broker details
	fmt.Printf("Subscribing to topic %s with TLS, with ca certificate %s  \n", topic, serverCAFile)

	tlsCfg, err := websocket.LoadTLS(certFile, keyFile, serverCAFile, clientCAFile)
	if err != nil {
		panic(err)
	}
	subClient, err := websocket.Connect(brokerAddress, tlsCfg)
	if err != nil {
		panic(err)
	}
	defer subClient.Disconnect(250)

	done := make(chan struct{}, 1)
	if token := subClient.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) { onMessage(c, m, done) }); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	fmt.Printf("Publishing to topic %s with TLS, with ca certificate %s  \n", topic, serverCAFile)
	pubClient, err := websocket.Connect(brokerAddress, tlsCfg)
	if err != nil {
		panic(err)
	}

	defer pubClient.Disconnect(250)

	pubClient.Publish(topic, 0, false, payload)
	<-done

	invalidPathBrokerAddress := brokerAddress + "/invalid_path"
	fmt.Printf("Publishing to topic %s with TLS, with ca certificate %s to invalid path %s \n", topic, serverCAFile, invalidPathBrokerAddress)
	pubClientInvalidPath, err := websocket.Connect(invalidPathBrokerAddress, tlsCfg)
	if err == nil {
		pubClientInvalidPath.Disconnect(250)
		panic("some thing went wrong")
	}
	fmt.Printf("Failed to connect with invalid path %s,error : %s\n", invalidPathBrokerAddress, err.Error())

	serverCAFile = ""
	fmt.Printf("Publishing to topic %s with TLS, without ca certificate %s  \n", topic, serverCAFile)
	tlsCfg, err = websocket.LoadTLS(certFile, keyFile, serverCAFile, clientCAFile)
	if err != nil {
		panic(err)
	}

	pubClientNoCerts, err := websocket.Connect(brokerAddress, tlsCfg)
	if err == nil {
		pubClientNoCerts.Disconnect(250)
		panic("some thing went wrong")
	}
	fmt.Printf("Failed to connect without Server certs,error : %s\n", err.Error())
}

func onMessage(_ mqtt.Client, m mqtt.Message, done chan struct{}) {
	fmt.Printf("Subscription Message Received, Topic : %s, Payload %s\n", m.Topic(), string(m.Payload()))
	done <- struct{}{}
}
