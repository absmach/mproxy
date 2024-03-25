#!/bin/bash
protocol=http
host=localhost
port=8086
path="messages"
content="application/json"
message="{\"message\": \"Hello mProxy\"}"
invalidPath="invalid_path"

echo "Posting message to ${protocol}://${host}:${port}/${path} without tls ..."
curl -sSiX POST "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN" -d "${message}"


echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} without tls and with basic authentication..."
curl -sSi -u username:password -X  POST "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -d "${message}"


echo -e "\nPosting message to invalid path ${protocol}://${host}:${port}/${invalidPath} without tls..."
curl -sSiX POST "${protocol}://${host}:${port}/${invalidPath}" -H "content-type:${content}" -H "Authorization:TOKEN" -d "${message}"
