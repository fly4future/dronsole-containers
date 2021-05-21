#!/bin/bash
docker run -it \
    -p 4560:4560 \
    -p 14560:14560/udp \
    -e DRONE_DEVICE_ID="uav1" \
    -e MQTT_BROKER_ADDRESS="tcp://<ip>:<port>" \
    ghcr.io/tiiuae/tii-fog-drone:f4f
