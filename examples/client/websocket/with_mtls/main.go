// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/absmach/mgate/examples/client/websocket"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	brokerAddress = "wss://localhost:8085/mgate-ws"
	topic         = "test/topic"
	payload       = "Hello mGate"
	certFile      = "ssl/certs/client.crt"
	keyFile       = "ssl/certs/client.key"
	serverCAFile  = "ssl/certs/ca.crt"
	clientCAFile  = ""
)

func main() {
	fmt.Printf("Subscribing to topic %s with mTLS, with ca certificate %s and with client certificate %s %s \n", topic, serverCAFile, certFile, keyFile)

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

	fmt.Printf("Publishing to topic %s with mTLS, with ca certificate %s and with client certificate %s %s \n", topic, serverCAFile, certFile, keyFile)
	pubClient, err := websocket.Connect(brokerAddress, tlsCfg)
	if err != nil {
		panic(err)
	}
	defer pubClient.Disconnect(250)

	pubClient.Publish(topic, 0, false, payload)
	<-done

	// Publisher with revoked certs
	certFile = "ssl/certs/client_revoked.crt"
	keyFile = "ssl/certs/client_revoked.key"
	fmt.Printf("Publishing to topic %s with mTLS, with ca certificate %s and with revoked client certificate %s %s \n", topic, serverCAFile, certFile, keyFile)
	tlsCfg, err = websocket.LoadTLS(certFile, keyFile, serverCAFile, clientCAFile)
	if err != nil {
		panic(err)
	}

	pubClient, err = websocket.Connect(brokerAddress, tlsCfg)
	if err == nil {
		pubClient.Disconnect(250)
		panic("some thing went wrong")
	}
	fmt.Printf("Failed to connect Publisher with revoked client certs,error : %s\n", err.Error())

	// Publisher with unknown certs
	certFile = "ssl/certs/client_unknown.crt"
	keyFile = "ssl/certs/client_unknown.key"
	fmt.Printf("Publishing to topic %s with mTLS, with ca certificate %s and with unknown client certificate %s %s \n", topic, serverCAFile, certFile, keyFile)
	tlsCfg, err = websocket.LoadTLS(certFile, keyFile, serverCAFile, clientCAFile)
	if err != nil {
		panic(err)
	}

	pubClient, err = websocket.Connect(brokerAddress, tlsCfg)
	if err == nil {
		pubClient.Disconnect(250)
		panic("some thing went wrong")
	}
	fmt.Printf("Failed to connect with unknown client certs,error : %s\n", err.Error())

	// Publisher with no client certs
	certFile = ""
	keyFile = ""
	fmt.Printf("Publishing to topic %s with mTLS, with ca certificate %s and without client certificate\n", topic, serverCAFile)
	tlsCfg1, err := websocket.LoadTLS(certFile, keyFile, serverCAFile, clientCAFile)
	if err != nil {
		panic(err)
	}

	pubClient, err = websocket.Connect(brokerAddress, tlsCfg1)
	if err == nil {
		pubClient.Disconnect(250)
		panic("some thing went wrong")
	}
	fmt.Printf("Failed to connect without client certs,error : %s\n", err.Error())

	// Publisher with no client certs
	serverCAFile = ""
	certFile = ""
	keyFile = ""
	fmt.Printf("Publishing to topic %s with mTLS, without ca certificate and without client certificate\n", topic)
	tlsCfg, err = websocket.LoadTLS(certFile, keyFile, serverCAFile, clientCAFile)
	if err != nil {
		panic(err)
	}

	pubClient, err = websocket.Connect(brokerAddress, tlsCfg)
	if err == nil {
		pubClient.Disconnect(250)
		panic("some thing went wrong")
	}
	fmt.Printf("Failed to connect without client certs,error : %s\n", err.Error())
}

func onMessage(_ mqtt.Client, m mqtt.Message, done chan struct{}) {
	fmt.Printf("Subscription Message Received, Topic : %s, Payload %s\n", m.Topic(), string(m.Payload()))
	done <- struct{}{}
}
