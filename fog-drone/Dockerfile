FROM ubuntu:20.04
# Setup timezone
RUN echo 'Etc/UTC' > /etc/timezone \
    && ln -s /usr/share/zoneinfo/Etc/UTC /etc/localtime \
    && apt-get update && apt-get install -q -y tzdata \
    && rm -rf /var/lib/apt/lists/*
# Install tools to add apt repository
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    gnupg2 \
    lsb-release \
    && rm -rf /var/lib/apt/lists/*

# Install ROS 2
RUN apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C1CF6E31E6BADE8868B172B4F42ED6FBAB17C654
RUN echo "deb [arch=$(dpkg --print-architecture)] http://packages.ros.org/ros2/ubuntu $(lsb_release -cs) main" > /etc/apt/sources.list.d/ros2-latest.list
RUN apt-get update && apt-get install -y --no-install-recommends \
    ros-foxy-ros-base \
    ros-foxy-ros-core \
    && rm -rf /var/lib/apt/lists/*

# Install gstreamer libraries for video streaming
RUN apt-get update && apt-get install -y --no-install-recommends \
    gstreamer1.0-0 \
    gstreamer1.0-plugins-base \
    gstreamer1.0-plugins-good \
    gstreamer1.0-plugins-bad \
    gstreamer1.0-rtsp \
    gstreamer1.0-tools \
    && rm -rf /var/lib/apt/lists/*

# Install git and ssh
RUN apt-get update && apt-get install -y --no-install-recommends \
    git-core \
    openssh-client \
    && rm -rf /var/lib/apt/lists/*

# Install MAVSDK
RUN curl -fsSL https://github.com/mavlink/MAVSDK/releases/download/v0.34.0/mavsdk_0.34.0_ubuntu20.04_amd64.deb -o mavsdk_0.34.0_ubuntu20.04_amd64.deb \
    && dpkg -i mavsdk_0.34.0_ubuntu20.04_amd64.deb \
    && rm mavsdk_0.34.0_ubuntu20.04_amd64.deb

# Install PX4 SITL
RUN curl -fsSL https://github.com/tiiuae/px4-firmware/releases/download/v1.11.0-1.0.4/px4_sitl_build.tar.gz -o px4_sitl_build.tar.gz \
    && tar -xzf px4_sitl_build.tar.gz \
    && mv px4_sitl/build/px4_sitl_rtps/bin/* /usr/bin/ \
    && mv px4_sitl/build/px4_sitl_rtps/etc /px4_sitl_etc \
    && rm -rf px4_sitl_build/ \
    && rm px4_sitl_build.tar.gz


## FOG SOFTWARE

# Install agent-protocol-splitter
RUN curl -fsSL https://github.com/tiiuae/fog_sw/releases/download/internal_rel_02-21/agent-protocol-splitter_1.0.0.624ae07_amd64.deb -o agent-protocol-splitter_1.0.0._amd64.deb \
    && dpkg -i agent-protocol-splitter_1.0.0._amd64.deb \
    && rm agent-protocol-splitter_1.0.0._amd64.deb

# Install latest communication-link (from container image)
COPY --from=ghcr.io/tiiuae/tii-communication-link-deb:latest /packages/*.deb .
RUN dpkg -i communication-link*.deb \
    && rm communication-link*.deb

# Install mavlink-router
RUN curl -fsSL https://github.com/tiiuae/fog_sw/releases/download/internal_rel_02-21/mavlink-router_2.0.0.2d095f3_amd64.deb -o mavlink-router_2.0.0._amd64.deb \
    && dpkg -i mavlink-router_2.0.0._amd64.deb \
    && rm mavlink-router_2.0.0._amd64.deb

# Install px4-mavlink-ctrl
RUN curl -fsSL https://github.com/tiiuae/fog_sw/releases/download/internal_rel_02-21/ros-foxy-px4-mavlink-ctrl_0.0.0-0focal_amd64.deb -o ros-foxy-px4-mavlink-ctrl_0.0.0-0focal_amd64.deb \
    && dpkg -i ros-foxy-px4-mavlink-ctrl_0.0.0-0focal_amd64.deb \
    && rm ros-foxy-px4-mavlink-ctrl_0.0.0-0focal_amd64.deb

# Install px4-msgs
RUN curl -fsSL https://github.com/tiiuae/fog_sw/releases/download/internal_rel_02-21/ros-foxy-px4-msgs_2.0.1-0focal_amd64.deb -o ros-foxy-px4-msgs_2.0.1-0focal_amd64.deb \
    && dpkg -i ros-foxy-px4-msgs_2.0.1-0focal_amd64.deb \
    && rm ros-foxy-px4-msgs_2.0.1-0focal_amd64.deb

# Install px4-ros-com
RUN curl -fsSL https://github.com/tiiuae/fog_sw/releases/download/internal_rel_02-21/ros-foxy-px4-ros-com_0.1.0-0focal_amd64.deb -o ros-foxy-px4-ros-com_0.1.0-0focal_amd64.deb \
    && dpkg -i ros-foxy-px4-ros-com_0.1.0-0focal_amd64.deb \
    && rm ros-foxy-px4-ros-com_0.1.0-0focal_amd64.deb

# Install latest mission-data-recorder
COPY --from=ghcr.io/kulmesa/tii-mission-data-recorder:latest /build/mission-data-recorder /usr/local/bin/

# Setup run environment
RUN mkdir /fog-drone
WORKDIR /fog-drone

COPY run-px4.sh               run-px4.sh
COPY run-ros2.sh              run-ros2.sh
COPY drone-entrypoint.sh      drone-entrypoint.sh

EXPOSE 14560/UDP
EXPOSE 4560

ENTRYPOINT ["/fog-drone/drone-entrypoint.sh"]
