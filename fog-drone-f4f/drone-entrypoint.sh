#!/bin/bash

mkdir /enclave
echo "$DRONE_IDENTITY_KEY" > /enclave/rsa_private.pem

~/tmux.sh
