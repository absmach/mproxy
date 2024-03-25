#!/bin/bash
protocol=https
host=localhost
port=8088
path="messages"
content="application/json"
message="{\"message\": \"Hello mProxy\"}"
invalidPath="invalid_path"
cafile=ssl/certs/ca.crt
certfile=ssl/certs/client.crt
keyfile=ssl/certs/client.key
reovokedcertfile=ssl/certs/client_revoked.crt
reovokedkeyfile=ssl/certs/client_revoked.key
unknowncertfile=ssl/certs/client_unknown.crt
unknownkeyfile=ssl/certs/client_unknown.key

echo "Posting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header, ca & client certificates ${cafile} ${certfile} ${keyfile}..."
curl -sSiX POST "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN" -d "${message}" --cacert $cafile --cert $certfile --key $keyfile

echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, basic authentication, ca & client certificates ${cafile} ${certfile} ${keyfile}..."
curl -sSi -u username:password -X  POST "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -d "${message}"  --cacert $cafile  --cert $certfile --key $keyfile

echo -e "\nPosting message to invalid path ${protocol}://${host}:${port}/${path}/${invalidPath} with tls, Authorization header, ca & client certificates ${cafile} ${certfile} ${keyfile}..."
curl -sSiX POST "${protocol}://${host}:${port}/${path}/${invalidPath}" -H "content-type:${content}" -H "Authorization:TOKEN" -d "${message}" --cacert $cafile  --cert $certfile --key $keyfile

echo -e "\nPosting message to invalid path ${protocol}://${host}:${port}/${invalidPath} with tls, Authorization header, ca & client certificates ${cafile} ${certfile} ${keyfile}..."
curl -sSiX POST "${protocol}://${host}:${port}/${invalidPath}" -H "content-type:${content}" -H "Authorization:TOKEN" -d "${message}" --cacert $cafile  --cert $certfile --key $keyfile

echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header, ca certificates ${cafile} & reovked client certificate ${reovokedcertfile} ${reovokedkeyfile}..."
curl -sSiX POST "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN" -d "${message}" --cacert $cafile  --cert $reovokedcertfile --key $reovokedkeyfile

echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header, ca certificates ${cafile} & unknown client certificate ${unknowncertfile} ${unknownkeyfile}..."
curl -sSiX POST "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN" -d "${message}" --cacert $cafile  --cert $unknowncertfile --key $unknownkeyfile

echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header, ca certificate ${cafile} & without client certificates.."
curl -sSiX POST "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN" -d "${message}" --cacert $cafile 2>&1

echo -e "\nPosting message to ${protocol}://${host}:${port}/${path} with tls, Authorization header,  & without ca , client certificates.."
curl -sSiX POST "${protocol}://${host}:${port}/${path}" -H "content-type:${content}" -H "Authorization:TOKEN" -d "${message}" 2>&1
