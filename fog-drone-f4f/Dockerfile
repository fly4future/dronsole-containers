FROM ros:foxy

# Setup run environment
RUN mkdir /fog-drone
WORKDIR /fog-drone

RUN sudo apt-get -y update 

# Install git and ssh
RUN apt-get install -y --no-install-recommends \
    git-core \
    openssh-client 

# Install tools
RUN apt-get install -y --no-install-recommends \
    curl \
    vim \
    ranger \
    tmux=3.0a-2

# install tmux config
COPY dottmux.conf /dottmux.conf
RUN /bin/sh -c 'mv /dottmux.conf ~/.tmux.conf'

RUN echo "deb [trusted=yes] https://artifactory.ssrc.fi/artifactory/debian-public-local focal fog-sw" >> /etc/apt/sources.list

# Install latest FOG_SW modules
RUN apt-get update && apt-get install -y --no-install-recommends \
    communication-link \
    mavlink-router \
    ros-foxy-px4-mavlink-ctrl \
    ros-foxy-px4-msgs \
    ros-foxy-px4-ros-com

# Install agent-protocol-splitter
RUN curl -fsSL https://github.com/tiiuae/fog_sw/releases/download/internal_rel_02-21/agent-protocol-splitter_1.0.0.624ae07_amd64.deb -o agent-protocol-splitter_1.0.0._amd64.deb \
    && dpkg -i agent-protocol-splitter_1.0.0._amd64.deb \
    && rm agent-protocol-splitter_1.0.0._amd64.deb

# Install MAVSDK
RUN curl -fsSL https://github.com/mavlink/MAVSDK/releases/download/v0.34.0/mavsdk_0.34.0_ubuntu20.04_amd64.deb -o mavsdk_0.34.0_ubuntu20.04_amd64.deb \
    && dpkg -i mavsdk_0.34.0_ubuntu20.04_amd64.deb \
    && rm mavsdk_0.34.0_ubuntu20.04_amd64.deb

# Install PX4 SITL
RUN curl -fsSL https://artifactory.ssrc.fi:443/artifactory/gen-release-local/builds/px4-firmware/sitl/px4_sitl_build-v1.11.0-rc3-2098-ge74cff132c.tar.gz -o px4_sitl_build.tar.gz \
    && tar -xzf px4_sitl_build.tar.gz \
    && mv px4_sitl/build/px4_sitl_rtps/bin/* /usr/bin/ \
    && mv px4_sitl/build/px4_sitl_rtps/etc /px4_sitl_etc \
    && rm -rf px4_sitl_build/ \
    && rm px4_sitl_build.tar.gz

# Install ROS2 dependencies
RUN apt-get install -y --no-install-recommends \
  ros-foxy-octomap \
  ros-foxy-octomap-msgs \
  ros-foxy-dynamic-edt-3d \
  ros-foxy-laser-geometry \
  ros-foxy-pcl-conversions \
  ros-foxy-pcl-msgs

# Install ROS2 f4f packages
COPY packages /packages
RUN cd /packages \
    && dpkg -i  *fog-msgs*.deb \
    && dpkg -i  *fog-core*.deb \
    && dpkg -i  *rplidar-ros2*.deb \
    && dpkg -i  *control-interface*.deb \
    && dpkg -i  *octomap-server2*.deb \
    && dpkg -i  *navigation*.deb \
    && if [ `ls -1 *.ddeb 2>/dev/null | wc -l ` -gt 0 ]; then dpkg -i *.ddeb; fi \
    && rm -rf /packages

# Prepare ~/.bashrc
RUN /bin/sh -c 'echo "source /opt/ros/foxy/setup.bash" >> ~/.bashrc'
RUN /bin/sh -c 'echo "export PX4_SIM_MODEL=ssrc_fog_x" >> ~/.bashrc'
 
COPY tmux.sh                  tmux.sh
COPY drone-entrypoint.sh      drone-entrypoint.sh

EXPOSE 14560/UDP
EXPOSE 4560

ENTRYPOINT ["/fog-drone/drone-entrypoint.sh"]
