#!/bin/bash
### BEGIN INIT INFO
# Provides: tmux
# Required-Start:    $local_fs $network dbus
# Required-Stop:     $local_fs $network
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: start the uav
### END INIT INFO
# if [ "$(id -u)" == "0" ]; then
#   exec sudo -u mrs "$0" "$@"
# fi

source $HOME/.bashrc

# do not change this
MAIN_DIR="/fog_drone/log_files"

# following commands will be executed first in each window
pre_input=""

# define commands
# 'name' 'command'
# DO NOT PUT SPACES IN THE NAMES
input=(
  'PX4' 'px4 -d "/px4_sitl_etc" -w sitl_'"${PX4_SIM_MODEL}"' -s /px4_sitl_etc/init.d-posix/rcS
'
  # 'mavlink_router' 'mavlink-routerd
# '
  'micrortps' 'micrortps_agent -t UDP -n '"$DRONE_DEVICE_ID"' 
'
  'control' 'ros2 launch control_interface control_interface.py use_sim_time:=true
'
  'sensors' 'ros2 launch fog_core static_tf_launch.py
'
  'navigation' 'ros2 launch navigation navigation.py use_sim_time:=true
'
  'octomap' 'ros2 launch octomap_server2 tii_rplidar_launch.py use_sim_time:=true
'
  'arm/takeoff' 'ros2 service call /'"$DRONE_DEVICE_ID"'/control_interface/arming std_srvs/srv/SetBool "data: true" && ros2 service call /'"$DRONE_DEVICE_ID"'/control_interface/takeoff std_srvs/srv/SetBool "data: true"'
  'land' 'ros2 service call /'"$DRONE_DEVICE_ID"'/control_interface/land std_srvs/srv/SetBool "data: true"'
  'goto' 'ros2 service call /'"$DRONE_DEVICE_ID"'/control_interface/local_setpoint fog_msgs/srv/Vec4 "goal: [0, 0, 2, 1]"'
)

init_window="PX4"

###########################
### DO NOT MODIFY BELOW ###
###########################

SESSION_NAME=uav

# prefere the user-compiled tmux
if [ -f /usr/local/bin/tmux ]; then
  export TMUX_BIN=/usr/local/bin/tmux
else
  export TMUX_BIN=/usr/bin/tmux
fi

# find the session
FOUND=$( $TMUX_BIN ls | grep $SESSION_NAME )

if [ $? == "0" ]; then

  echo "The session already exists"
  exit
fi

# Absolute path to this script. /home/user/bin/foo.sh
SCRIPT=$(readlink -f $0)
# Absolute path this script is in. /home/user/bin
SCRIPTPATH=`dirname $SCRIPT`

if [ -z ${TMUX} ];
then
  TMUX= $TMUX_BIN new-session -s "$SESSION_NAME" -d
  echo "Starting new session."
else
  echo "Already in tmux, leave it first."
  exit
fi

# create file for logging terminals' output
mkdir -p "$MAIN_DIR"
SUFFIX=$(date +"%Y_%m_%d_%H_%M_%S")
TMUX_DIR="$MAIN_DIR/tmux"
mkdir -p "$TMUX_DIR"

# create arrays of names and commands
for ((i=0; i < ${#input[*]}; i++));
do
  ((i%2==0)) && names[$i/2]="${input[$i]}"
  ((i%2==1)) && cmds[$i/2]="${input[$i]}"
done

# run tmux windows
for ((i=0; i < ${#names[*]}; i++));
do
  $TMUX_BIN new-window -t $SESSION_NAME:$(($i+1)) -n "${names[$i]}"
done

sleep 3

# start loggers
for ((i=0; i < ${#names[*]}; i++));
do
  $TMUX_BIN pipe-pane -t $SESSION_NAME:$(($i+1)) -o "ts | cat >> $TMUX_DIR/$(($i+1))_${names[$i]}.log"
done

# send commands
for ((i=0; i < ${#cmds[*]}; i++));
do
  $TMUX_BIN send-keys -t $SESSION_NAME:$(($i+1)) "cd $SCRIPTPATH;
done

# identify the index of the init window
init_index=0
for ((i=0; i < ((${#names[*]})); i++));
do
  if [ ${names[$i]} == "$init_window" ]; then
    init_index=$(expr $i + 1)
  fi
done

$TMUX_BIN select-window -t $SESSION_NAME:$init_index

$TMUX_BIN -2 attach-session -t $SESSION_NAME

clear