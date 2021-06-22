#!/bin/sh
make generate manifest
img=192.168.9.12:5000/hypercloud-multi-operator-feature:v$1
make docker-build IMG=$img
make docker-push IMG=$img 
