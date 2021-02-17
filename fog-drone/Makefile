ARCH := $(shell dpkg --print-architecture)
RELEASE := $(shell lsb_release -cs)

install:
	#apt update

	# Setup timezone
	echo 'Etc/UTC' > /etc/timezone
	(test ! -f /etc/localtime && ln -s /usr/share/zoneinfo/Etc/UTC /etc/localtime) || true
	apt install -q -y tzdata

	# Install tools to add apt repository
	apt install -y --no-install-recommends curl gnupg2 lsb-release

	# Install ROS 2
	apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C1CF6E31E6BADE8868B172B4F42ED6FBAB17C654
	echo "deb [arch=$(ARCH)] http://packages.ros.org/ros2/ubuntu $(RELEASE) main" > /etc/apt/sources.list.d/ros2-latest.list
	apt update
	apt install -y --no-install-recommends ros-foxy-ros-base ros-foxy-ros-core

	apt install -y --no-install-recommends libgstreamer1.0-0 gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad gstreamer1.0-rtsp
	# NOT FOUND! gstreamer1.0-0 using libgstreamer1.0-0

	# Install git and ssh
	apt install -y --no-install-recommends git-core openssh-client

	# Install MAVSDK
	curl -fsSL https://github.com/mavlink/MAVSDK/releases/download/v0.34.0/mavsdk_0.34.0_ubuntu20.04_amd64.deb -o mavsdk_0.34.0_ubuntu20.04_amd64.deb
	dpkg -i mavsdk_0.34.0_ubuntu20.04_amd64.deb
	rm mavsdk_0.34.0_ubuntu20.04_amd64.deb

	# Install PX4 SITL
	curl -fsSL https://github.com/tiiuae/px4-firmware/releases/download/ver_ssrc_rel_2/px4_sitl_build.tar.gz -o px4_sitl_build.tar.gz
	tar -xzf px4_sitl_build.tar.gz
	mv px4_sitl/build/px4_sitl_rtps/bin/* /usr/bin/
	mv px4_sitl/build/px4_sitl_rtps/etc /px4_sitl_etc
	rm -rf px4_sitl_build/
	rm px4_sitl_build.tar.gz


	## FOG SOFTWARE

	# Install agent-protocol-splitter
	curl -fsSL https://github.com/tiiuae/fog_sw/releases/download/ver_ssrc_rel_2/agent-protocol-splitter_1.0.0.624ae07_amd64.deb -o agent-protocol-splitter_1.0.0._amd64.deb
	dpkg -i agent-protocol-splitter_1.0.0._amd64.deb
	rm agent-protocol-splitter_1.0.0._amd64.deb

	# Install latest communication-link (from container image)
	### NO MAKE EQUIVALENT FOR COPY FROM INSIDE ANOTHER IMAGE
	#COPY --from=ghcr.io/tiiuae/tii-communication-link-deb:latest /packages/*.deb .
	#RUN dpkg -i communication-link*.deb \
	#rm communication-link*.deb

	# Install mavlink-router
	curl -fsSL https://github.com/tiiuae/fog_sw/releases/download/ver_ssrc_rel_2/mavlink-router_2.0.0.2d095f3_amd64.deb -o mavlink-router_2.0.0._amd64.deb
	dpkg -i mavlink-router_2.0.0._amd64.deb
	rm mavlink-router_2.0.0._amd64.deb

	# Install px4-mavlink-ctrl
	curl -fsSL https://github.com/tiiuae/fog_sw/releases/download/ver_ssrc_rel_2/ros-foxy-px4-mavlink-ctrl_0.0.0-0focal_amd64.deb -o ros-foxy-px4-mavlink-ctrl_0.0.0-0focal_amd64.deb
	dpkg -i ros-foxy-px4-mavlink-ctrl_0.0.0-0focal_amd64.deb
	rm ros-foxy-px4-mavlink-ctrl_0.0.0-0focal_amd64.deb

	# Install px4-msgs
	curl -fsSL https://github.com/tiiuae/fog_sw/releases/download/ver_ssrc_rel_2/ros-foxy-px4-msgs_2.0.1-0focal_amd64.deb -o ros-foxy-px4-msgs_2.0.1-0focal_amd64.deb
	dpkg -i ros-foxy-px4-msgs_2.0.1-0focal_amd64.deb
	rm ros-foxy-px4-msgs_2.0.1-0focal_amd64.deb

	# Install px4-ros-com
	curl -fsSL https://github.com/tiiuae/fog_sw/releases/download/ver_ssrc_rel_2/ros-foxy-px4-ros-com_0.1.0-0focal_amd64.deb -o ros-foxy-px4-ros-com_0.1.0-0focal_amd64.deb
	dpkg -i ros-foxy-px4-ros-com_0.1.0-0focal_amd64.deb
	rm ros-foxy-px4-ros-com_0.1.0-0focal_amd64.deb

	# Setup run environment
	mkdir /fog-drone

	cp run-px4.sh               /fog-drone/run-px4.sh
	cp run-ros2.sh              /fog-drone/run-ros2.sh
	cp drone-entrypoint.sh      /fog-drone/drone-entrypoint.sh
