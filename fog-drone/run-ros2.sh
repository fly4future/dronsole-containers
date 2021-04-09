#!/bin/bash

source /opt/ros/foxy/setup.bash
echo "Start Mavlink Router"
mavlink-routerd >/fog-drone/mav_routerd_out.log 2>/fog-drone/mav_routerd_err.log &
echo "Start Mavlink control"
ros2 launch px4_mavlink_ctrl mavlink_ctrl.launch >/fog-drone/mav_ctrl_out.log 2>/fog-drone/mav_ctrl_err.log &
echo "Start Micrortps_agent"
micrortps_agent -t UDP -n "$DRONE_DEVICE_ID" >/fog-drone/urtps_out.log 2>/fog-drone/urtps_err.log &
echo "Start videonode"
videonode -stream-address "rtsp://$RTSP_SERVER_ADDRESS/$DRONE_DEVICE_ID" >/fog-drone/videonode_out.log 2>/fog-drone/videonode_err.log &
if [ "$RECORD_MISSION_DATA" = true ]; then
    echo "Start mission data recorder"
    mission-data-recorder \
        -device-id "$DRONE_DEVICE_ID" \
        -backend-url "$MISSION_DATA_RECORDER_BACKEND_URL" \
        -size-threshold "$MISSION_DATA_RECORDER_SIZE_THRESHOLD" \
        -topics "$MISSION_DATA_RECORDER_TOPICS" \
        -dest-dir /fog-drone/mission-data \
        >/fog-drone/mission-data-recorder_out.log \
        2>/fog-drone/mission-data-recorder_err.log &
else
    echo "Mission data recording was not requested"
fi
echo "Start Communication link"
communication_link -device_id "$DRONE_DEVICE_ID" -mqtt_broker "$MQTT_BROKER_ADDRESS"
