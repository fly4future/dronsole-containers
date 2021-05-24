#!/bin/bash

source /opt/ros/foxy/setup.bash
echo "Start Mavlink Router"
mavlink-routerd >/fog-drone/mav_routerd_out.log 2>/fog-drone/mav_routerd_err.log &
echo "Start Mavlink control"
ros2 launch px4_mavlink_ctrl mavlink_ctrl.launch >/fog-drone/mav_ctrl_out.log 2>/fog-drone/mav_ctrl_err.log &
echo "Start Micrortps_agent"
micrortps_agent -t UDP -n "$DRONE_DEVICE_ID" >/fog-drone/urtps_out.log 2>/fog-drone/urtps_err.log &
echo "Start video"
gst-launch-1.0 udpsrc port=5600 ! "application/x-rtp" ! rtph264depay ! queue ! video/x-h264 ! rtspclientsink name=sink protocols=tcp location="rtsp://$RTSP_SERVER_ADDRESS/$DRONE_DEVICE_ID" tls-validation-flags=generic-error >/fog-drone/video_out.log 2>/fog-drone/video_err.log &
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
echo "Start Mission Engine"
mission-engine -device_id "$DRONE_DEVICE_ID" >/fog-drone/mission-engine_out.log 2>/fog-drone/mission-engine_err.log &
echo "Start Communication link"
communication_link -device_id "$DRONE_DEVICE_ID" -mqtt_broker "$MQTT_BROKER_ADDRESS"
