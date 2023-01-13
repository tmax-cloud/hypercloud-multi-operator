#!/bin/bash
source setting.conf

./push.sh
# kubectl create ns hypercloud5-system
make deploy-test
