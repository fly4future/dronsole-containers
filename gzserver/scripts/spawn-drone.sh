#!/bin/bash

if [[ $# -lt 11 ]]; then
    echo "Too few arguments!"
    echo "$0 <mavlink_addr> <mavlink_udp_port> <mavlink_tcp_port> <video_udp_port> <name> <pos_x> <pos_y> <pos_z> <yaw> <pitch> <roll>"
    exit 1
fi

mav_addr=$1
name=$5
pos_x=$6
pos_y=$7
pos_z=$8
yaw=$9
pitch=${10}
roll=${11}

source /opt/ros/foxy/setup.bash

export PX4_SIM_MODEL=ssrc_fog_x

mavlink_udp_port=$2
mavlink_tcp_port=$3
video_udp_port=$4

export GAZEBO_PLUGIN_PATH=$GAZEBO_PLUGIN_PATH:/data/plugins
export GAZEBO_MODEL_PATH=$GAZEBO_MODEL_PATH:/data/models
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/data/plugins
export PATH=$PATH:/data/plugins

echo "Starting instance $name"

python3 /data/scripts/jinja_gen.py /data/models/${PX4_SIM_MODEL}/${PX4_SIM_MODEL}.sdf.jinja /data \
        --use_tcp 0 \
        --mavlink_addr "${mav_addr}" \
        --mavlink_udp_port "${mavlink_udp_port}" \
        --mavlink_tcp_port "${mavlink_tcp_port}" \
        --gstudphost "${mav_addr}" \
        --gstudpport "${video_udp_port}" \
        --namespace "${name}" \
        --output-file "/tmp/${PX4_SIM_MODEL}_${name}.sdf" || exit

echo "Spawning ${PX4_SIM_MODEL}_${name}"

gz model --spawn-file="/tmp/${PX4_SIM_MODEL}_${name}.sdf" \
         --model-name="${PX4_SIM_MODEL}_${name}" \
         -x "${pos_x}" \
         -y "${pos_y}" \
         -z "${pos_z}" \
         -Y "${yaw}" \
         -P "${pitch}" \
         -R "${roll}" || exit

echo "Done"
