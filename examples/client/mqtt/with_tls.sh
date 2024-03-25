#!/bin/bash

topic="test/topic"
message="Hello mProxy"
host=localhost
port=8883
cafile=ssl/certs/ca.crt

echo "Subscribing to topic ${topic} with TLS certifcate ${cafile}..."
mosquitto_sub -h $host -p $port -t $topic --cafile $cafile &
sub_pid=$!
sleep 1

cleanup() {
    echo "Cleaning up..."
    kill $sub_pid
}

trap cleanup EXIT

echo "Publishing to topic ${topic} with TLS, with ca certificate ${cafile}..."
mosquitto_pub -h $host -p $port -t $topic  -m "${message}"  --cafile $cafile
sleep 1


echo "Publishing to topic ${topic} with TLS, without ca certificate ...."
mosquitto_pub -h $host -p $port -t $topic -m "${message}"  2>&1
sleep 1
