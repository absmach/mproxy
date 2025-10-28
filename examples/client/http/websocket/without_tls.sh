#!/bin/bash
protocol=ws
host=localhost
port=8086
path="mgate-http/messages/ws"
content="application/json"
message="{\"message\": \"Hello mGate\"}"
invalidPath="invalid_path"

echo "Posting message to ${protocol}://${host}:${port}/${path} without tls ..."
echo "${message}"  | websocat  "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN"


echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} without tls and with basic authentication..."
echo "${message}"  | websocat --basic-auth "${protocol}://${host}:${port}/${path}" -H "content-type:${content}"


echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} without tls and with authentication in query params..."
echo "${message}"  | websocat  "${protocol}://${host}:${port}/${path}?authorization=TOKEN" -H "content-type:${content}"


echo -e "\nPosting message to invalid path ${protocol}://${host}:${port}/${invalidPath} without tls..."
echo "${message}"  | websocat  "${protocol}://${host}:${port}/${invalidPath}" -H "content-type:${content}" -H "Authorization:TOKEN"
