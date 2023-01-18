#!/bin/sh
echo "IMG=${IMG}"
make generate manifests
make docker-build IMG=$IMG
make docker-push IMG=$IMG
