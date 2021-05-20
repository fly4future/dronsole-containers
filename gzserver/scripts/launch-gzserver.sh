#!/bin/bash

if [[ $# -lt 1 ]]; then
    echo "Too few arguments!"
    echo "$0 <world_file>"
    exit 1
fi

world_file=$1

# For Gstreamer camera
export DISPLAY=:1.0
Xvfb :1 -screen 0 1600x1200x16 &

export PX4_SIM_MODEL=ssrc_fog_x
source /usr/share/gazebo/setup.sh
source /opt/ros/foxy/setup.bash

# setup Gazebo env and update package path
export GAZEBO_PLUGIN_PATH=$GAZEBO_PLUGIN_PATH:/data/plugins
export GAZEBO_MODEL_PATH=$GAZEBO_MODEL_PATH:/data/models
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/data/plugins

echo "Starting gazebo"
IP_ADDR=$(hostname -I)
GAZEBOIP=${IP_ADDR} GAZEBOMASTER_URI=${IP_ADDR}:11345 gzserver "$world_file" --verbose
