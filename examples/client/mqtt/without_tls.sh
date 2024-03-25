#!/bin/bash

topic="test/topic"
message="Hello mProxy"
host=localhost
port=1884

echo "Subscribing to topic ${topic} without TLS..."
mosquitto_sub -h $host -p $port -t $topic &
sub_pid=$!
sleep 1

cleanup() {
    echo "Cleaning up..."
    kill $sub_pid
}

# Trap the EXIT and ERR signals and call the cleanup function
trap cleanup EXIT

echo "Publishing to topic ${topic} without TLS..."
mosquitto_pub -h $host -p $port -t $topic -m "${message}"
sleep 1
