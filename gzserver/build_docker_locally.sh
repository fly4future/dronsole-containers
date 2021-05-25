#!/bin/bash
# get the path to this script
MY_PATH=`dirname "$0"`
MY_PATH=`( cd "$MY_PATH" && pwd )`
cd "$MY_PATH"

eval $(minikube docker-env)
docker build -t ghcr.io/tiiuae/tii-gzserver .
