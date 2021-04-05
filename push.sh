#!/bin/sh
make generate manifests
img=192.168.6.197:5000/hypercloud-multi-operator:v7.3
make docker-build IMG=$img
make docker-push IMG=$img 
