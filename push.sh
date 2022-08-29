#!/bin/sh
# make generate manifests
img=docker.io/tmaxcloudck/hypercloud-multi-operator:$1
make docker-build IMG=$img
make docker-push IMG=$img 