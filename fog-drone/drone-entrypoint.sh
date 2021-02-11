#!/bin/bash

test -z "$MQTT_BROKER_ADDRESS" && echo "MQTT_BROKER_ADDRESS is not defined!" && exit 201
test -z "$DRONE_DEVICE_ID" && echo "DRONE_DEVICE_ID is not defined!" && exit 202
test -z "$DRONE_IDENTITY_KEY" && echo "DRONE_IDENTITY_KEY is not defined!" && exit 203

mkdir /enclave
echo "$DRONE_IDENTITY_KEY" > /enclave/rsa_private.pem

/fog-drone/run-px4.sh
/fog-drone/run-ros2.sh
