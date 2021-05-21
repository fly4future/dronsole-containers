#!/bin/bash

mkdir /enclave
echo "$DRONE_IDENTITY_KEY" > /enclave/rsa_private.pem

bash /fog-drone/tmux.sh

while sleep 1; do
  tmux has-session -t "uav" 2>/dev/null

  if [ $? != 0 ]; then
    echo "The tmux session doesn't exist anymore. Closing endpoint script."
    break
  fi
done

# infinite loop
# while [ 1 ]                                                                
# do                                                                         
#   sleep 60                    
# done
