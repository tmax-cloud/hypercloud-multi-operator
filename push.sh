#!/bin/sh
branch=$(git symbolic-ref --short -q HEAD)
tag=$(curl -X GET http://192.168.9.12:5000/v2/hypercloud-multi-operator-dev-shkim/tags/list 2>/dev/null \
    | jq '.tags[-1]' \
    | sed 's/\"v//' \
    | sed 's/.0\"//')
make generate manifest
img=192.168.9.12:5000/hypercloud-multi-operator-${branch}:v$((${tag} + 1)).0
make docker-build IMG=$img
make docker-push IMG=$img