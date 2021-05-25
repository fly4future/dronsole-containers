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
    ros-foxy-action-msgs \
    ros-foxy-actionlib-msgs \
    ros-foxy-builtin-interfaces \
    ros-foxy-diagnostic-msgs \
    ros-foxy-example-interfaces \
    ros-foxy-geometry-msgs \
    ros-foxy-libyaml-vendor \
    ros-foxy-lifecycle-msgs \
    ros-foxy-map-msgs \
    ros-foxy-nav-msgs \
    ros-foxy-pcl-msgs \
    ros-foxy-pendulum-msgs \
    ros-foxy-rcl \
    ros-foxy-rcl-interfaces \
    ros-foxy-rcl-logging-spdlog \
    ros-foxy-rcl-yaml-param-parser \
    ros-foxy-rcpputils \
    ros-foxy-rcutils \
    ros-foxy-rmw \
    ros-foxy-rmw-dds-common \
    ros-foxy-rmw-implementation \
    ros-foxy-ros-base \
    ros-foxy-ros-core \
    ros-foxy-rosgraph-msgs \
    ros-foxy-rosidl-runtime-c \
    ros-foxy-rosidl-typesupport-c \
    ros-foxy-sensor-msgs \
    ros-foxy-shape-msgs \
    ros-foxy-statistics-msgs \
    ros-foxy-std-msgs \
    ros-foxy-stereo-msgs \
    ros-foxy-test-msgs \
    ros-foxy-tf2-msgs \
    ros-foxy-tracetools \
    ros-foxy-trajectory-msgs \
    ros-foxy-turtlesim \
    ros-foxy-unique-identifier-msgs \
    ros-foxy-visualization-msgs \
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

# Install PX4 SITL
RUN curl -fsSL https://github.com/tiiuae/px4-firmware/releases/download/v1.11.0-1.0.4/px4_sitl_build.tar.gz -o px4_sitl_build.tar.gz \
    && tar -xzf px4_sitl_build.tar.gz \
    && mv px4_sitl/build/px4_sitl_rtps/bin/* /usr/bin/ \
    && mv px4_sitl/build/px4_sitl_rtps/etc /px4_sitl_etc \
    && rm -rf px4_sitl_build/ \
    && rm px4_sitl_build.tar.gz

## FOG SOFTWARE

RUN echo "deb [trusted=yes] https://artifactory.ssrc.fi/artifactory/debian-public-local $(lsb_release -cs) fog-sw" > /etc/apt/sources.list.d/fogsw-latest.list
RUN apt-get update && apt-get install -y --no-install-recommends \
    mavsdk \
    communication-link \
    ros-foxy-px4-msgs \
    agent-protocol-splitter \
    mavlink-router \
    ros-foxy-px4-mavlink-ctrl \
    ros-foxy-px4-ros-com \
    mission-engine \
    && rm -rf /var/lib/apt/lists/*

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
