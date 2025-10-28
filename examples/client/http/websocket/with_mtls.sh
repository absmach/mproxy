#!/bin/bash
protocol=wss
host=localhost
port=8088
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

echo "Posting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header, ca & client certificates ${cafile} ${certfile} ${keyfile}..."
echo "${message}" | websocat --binary --ws-c-uri="${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN" - ws-c:cmd:"openssl s_client -connect ${host}:${port} -quiet -verify_quiet -CAfile ${cafile} -cert ${certfile} -key ${keyfile}"


echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, basic authentication ca & client certificates ${cafile} ${certfile} ${keyfile}..."
encoded=$(printf "username:password" | base64)
echo "${message}" | websocat --binary --ws-c-uri="${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization: Basic $encoded" - ws-c:cmd:"openssl s_client -connect ${host}:${port} -quiet -verify_quiet -CAfile ${cafile} -cert ${certfile} -key ${keyfile}"

echo -e "\nPosting message to invalid path ${protocol}://${host}:${port}/${path}/${invalidPath} with tls, Authorization header, ca & client certificates ${cafile} ${certfile} ${keyfile}..."
echo "${message}" | websocat --binary --ws-c-uri="${protocol}://${host}:${port}/${invalidPath}" -H "content-type:${content}" -H "Authorization:TOKEN" - ws-c:cmd:"openssl s_client -connect ${host}:${port} -quiet -verify_quiet -CAfile ${cafile} -cert ${certfile} -key ${keyfile}"

echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header, ca certificates ${cafile} & reovked client certificate ${reovokedcertfile} ${reovokedkeyfile}..."
echo "${message}" | websocat --binary --ws-c-uri="${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN" - ws-c:cmd:"openssl s_client -connect ${host}:${port} -quiet -verify_quiet -CAfile ${cafile} -cert ${reovokedcertfile} -key ${reovokedkeyfile}"

echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header, ca certificates ${cafile} & unknown client certificate ${unknowncertfile} ${unknownkeyfile}..."
echo "${message}" | websocat --binary --ws-c-uri="${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN" - ws-c:cmd:"openssl s_client -connect ${host}:${port} -quiet -verify_quiet -CAfile ${cafile} -cert ${unknowncertfile} -key ${unknownkeyfile}"

echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header, ca certificate ${cafile} & without client certificates.."
echo "${message}" | websocat --binary --ws-c-uri="${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN" - ws-c:cmd:"openssl s_client -connect ${host}:${port} -quiet -verify_quiet -CAfile ${cafile}"

echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header,  & without ca , client certificates.."
echo "${message}" | websocat --binary --ws-c-uri="${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN" - ws-c:cmd:"openssl s_client -connect ${host}:${port} -quiet -verify_quiet"
