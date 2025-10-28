#!/bin/bash
protocol=wss
host=localhost
port=8087
path="mgate-http/messages/ws"
content="application/json"
message="{\"message\": \"Hello mGate\"}"
invalidPath="invalid_path"
cafile=ssl/certs/ca.crt
certfile=ssl/certs/client.crt
keyfile=ssl/certs/client.key
reovokedcertfile=ssl/certs/client_revoked.crt
reovokedkeyfile=ssl/certs/client_revoked.key
unknowncertfile=ssl/certs/client_unknown.crt
unknownkeyfile=ssl/certs/client_unknown.key

echo "Posting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header, ca certificate ${cafile}..."
# echo "${message}" | websocat  -H "content-type:${content}" -H "Authorization:TOKEN" --binary --ws-c-uri="${protocol}://${host}:${port}/${path}" - ws-c:cmd:"openssl s_client -connect ${host}:${port} -quiet -verify_quiet -CAfile ${cafile}"
echo "${message}" | SSL_CERT_FILE="${cafile}"  websocat  "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN"


echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, basic authentication ca certificate ${cafile}...."
encoded=$(printf "username:password" | base64)
echo "${message}" | SSL_CERT_FILE="${cafile}" websocat "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization: Basic $encoded"


echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header, and without ca certificate.."
echo "${message}" |  websocat "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization: Basic $encoded"

