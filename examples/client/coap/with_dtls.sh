#!/bin/bash
protocol=coaps
host=localhost
port=5684
path="test"
content=0x32
message="{\"message\": \"Hello mGate\"}"
auth="TOKEN"
cafile=ssl/certs/ca.crt
certfile=ssl/certs/client.crt
keyfile=ssl/certs/client.key

echo "Posting message to ${protocol}://${host}:${port}/${path} with dtls ..."
coap-client -m post coap://${host}:${port}/${path} -e "${message}" -O 12,${content} -O 15,auth=${auth} \
    -c $certfile -k $keyfile -C $cafile

echo "Getting message from ${protocol}://${host}:${port}/${path} with dtls ..."
coap-client -m get coaps://${host}:${port}/${path} -O 6,0x00 -O 15,auth=${auth} -c $certfile -k $keyfile -C $cafile

echo "Posting message to ${protocol}://${host}:${port}/${path} with dtls and invalid client certificate..."
coap-client -m post ${protocol}://${host}:${port}/${path} -e "${message}" -O 12,${content} -O 15,auth=${auth} \
    -c ssl/certs/client_unknown.crt -j ssl/certs/client_unknown.key -C "$cafile"

echo "Getting message from ${protocol}://${host}:${port}/${path} with dtls and invalid client certificate..."
coap-client -m get ${protocol}://${host}:${port}/${path} -O 6,0x00 -O 15,auth=${auth} -c ssl/certs/client_unknown.crt -j ssl/certs/client_unknown.key -C "$cafile"
