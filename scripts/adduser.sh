#!/bin/bash

# $1: cluster user name
# $2: temp password
# $3: public key file content\

if [ $# -eq 0 ]
  then
    >&2 echo "No arguments supplied"
    exit 1
fi

set -e

echo "Adding user $1"
sudo adduser --disabled-password --gecos "" $1

sudo echo "$1:$2" | sudo chpasswd

sudo usermod -aG sudo $1

sudo echo "${1} ALL=(ALL:ALL) NOPASSWD: ALL" | sudo EDITOR="tee -a" visudo

passwd -l root

mkdir -p /home/$1/.ssh

chown $1 /home/$1/.ssh
chmod 700 /home/$1/.ssh
touch /home/$1/.ssh/authorized_keys
chown $1 /home/$1/.ssh/authorized_keys
chmod 600 /home/$1/.ssh/authorized_keys
echo $3 | tee /home/$1/.ssh/authorized_keys

sed -i "s/#PasswordAuthentication yes/PasswordAuthentication no/g" /etc/ssh/sshd_config
sudo service ssh restart