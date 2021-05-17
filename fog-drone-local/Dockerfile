FROM ros:foxy


# Install tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
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

RUN echo "deb [trusted=yes] https://artifactory.ssrc.fi/artifactory/debian-public-local focal fog-sw" >> /etc/apt/sources.list

# Install latest FOG_SW modules
RUN apt-get update && apt-get install -y --no-install-recommends \
    communication-link \
    mavlink-router \
    ros-foxy-px4-mavlink-ctrl \
    ros-foxy-px4-msgs \
    ros-foxy-px4-ros-com \
    && rm -rf /var/lib/apt/lists/*

# Install PX4 SITL
RUN curl -fsSL https://artifactory.ssrc.fi:443/artifactory/gen-release-local/builds/px4-firmware/sitl/px4_sitl_build-v1.11.0-rc3-2098-ge74cff132c.tar.gz -o px4_sitl_build.tar.gz \
    && tar -xzf px4_sitl_build.tar.gz \
    && mv px4_sitl/build/px4_sitl_rtps/bin/* /usr/bin/ \
    && mv px4_sitl/build/px4_sitl_rtps/etc /px4_sitl_etc \
    && rm -rf px4_sitl_build/ \
    && rm px4_sitl_build.tar.gz


## FOG SOFTWARE

COPY packages /packages
RUN cd /packages \
    && if [ -e *.deb ]; then dpkg -i *.deb; fi

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
