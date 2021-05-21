#!/bin/bash
# get the path to this script
MY_PATH=`dirname "$0"`
MY_PATH=`( cd "$MY_PATH" && pwd )`
cd "$MY_PATH"

docker build -t ghcr.io/tiiuae/tii-fog-drone:f4f .
