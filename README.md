# Overview

`swarmgo` allows to build swarm clusters with monitoring, alerting, and https access (using ACME provider) in a few minutes

# Status

The project is widely used internally, not quire ready for public usage

- Security should be improved, now the process of adding nodes to the cluster requires `NOPASSWD: ALL` option for sudo
- traefik runs as a single instance, so resulting cluster is not fault tolerant anymore (some downtime will take place if traefik node goes down)
  - "Single instance" is a result of switching to traefik 2+ version, which broke compatibility with previous setup

# Usage

Prerequestes:

- Go 1.16
- sh
- git
- Need opportunity to use sudo command on all nodes without password:
  - use `sudo visudo`
  - modify `%sudo   ALL=(ALL:ALL) ALL` to `%sudo   ALL=(ALL:ALL) NOPASSWD: ALL`

Steps:

- Fork swarmgo repo to `mycluster`
- git clone `mycluster`
- Run `go build swarmgo.go` to build swarmgo executable
- Run `swarmgo init` to init config
  - Command creates `nodes/swarmgo-config.yml` with the swarmgo configuration settings
  - Setting in this configuration file may be updated:
    - When building cluster with LetsEncrypt certificate, make sure that `ACMEEnabled` setting is set to `true`, also `Domain` and `Email` settings filled properly
    - When target nodes are already pre-configured for SSH access with private/public keys, make sure that `PublicKey` and `PrivateKey` settings are filled properly
- Run `swarmgo keys` to generate new SSH keys. This is only needed to be execited with target node(s) is pre-configured with plaintext password. Skip this option when node is already pre-configured with key access for SSH.
  - Keys are kept in `nodes/swarmgo-config.yml` 
- Run ``eval `swarmgo agent` ``
  - Command starts `ssh-agent` enabling single sign-on in the current terminal session. SSH keys must be configured.
- Run `swarmgo imlucky IP1 [IP2] [IP3]` to build cluster automatically, with settings assigned automatically
  - Nodes will be added with aliases node1, node2, node3
  - One or three nodes will be assigned as managers, depending on number of nodes
  - Traefik and consul will be installed
  - Use option `-s` when ClusterUser already exists and SSH access using private/public keys is already configured on nodes being added. Make sure that SSH keys configured in `swarmgo-config.yml` when using this option.
  - Use option `-p password` to specify root password (password access will be disabled)
  - Use option `-m password` to specify password for authentication in monitoring services (Grafana, Prometheus, Traefik dashboard and Alert Manager)
  - Use `-n` option to disable alerts from alertmanager
  - Use `-w webhook_url` option to configure Slack alerts for specified webhook URL

  Example: `swarmgo imlucky 192.168.98.10 192.168.98.11 192.168.98.12 -p "pas" -m "mon" -n -s`

Commands to build cluster manually:

- Run `swarmgo add <Alias1>=<IP1> <Alias2>=<IP2>`
  - All node are kept in `nodes.yml`
  - Note: won't be possible to use password anymore
  - Node will be added to `nodes` file
  - Use `swarmgo add -p password` option to specify root password
  - Use `swarmgo add -s` option when user specified as `ClusterUser` in `swarmgo-config.yml` already exists and SSH access is configured for on nodes being added. 
- Run `swarmgo docker`
  - Install docker to all nodes which do not have docker installed yet (ref. `nodes.yml`)
- Run `swarmgo swarm -m <Alias1> <Alias2>`
  - Install swarm `manager` modes
- Run `swarmgo swarm`
  - Install swarm in `worker` mode for all nodes which do not have swarm configured yet
  - At least one manager must be configured first
- Run `swarmgo traefik` to deploy traefik
  - Use `-p password` option to specify password for Traefik dashboard web-ui
- Run `swarmgo label add [alias] [label]` to add node labels
  - example: `swarmgo label add node1 prometheus=true` to label node for deploying Prometheus service
- Run `swarmgo mon` to install prometheus, alertmanager, cadvisor and grafana
  - There must be one node with label `prometheus=true`, use `swarmgo label add` to add it
  - Use `-n` option to disable alerts
  - Use `-s webhook_url` to configure Slack alerts for specified webhook URL
  - Use `-u` option to update alertmanager settings, can be combined with `-n` or `-s` options
  - Use `-a password` option to specify Alertmanager password
  - Use `-p password` option to specify Prometheus password
  - Use `-g password` option to specify Grafana password

Services:
- mycluster.io/dashboard - Traefik dashboard
- mycluster.io/grafana
- mycluster.io/prometheus
- mycluster.io/alertmanager

# Developer Guide

- Create `.nodes` folder to keep nodes related files
  - This filder will be choosen automatically
  - This folder is ignored by git

# Under the Hood

Networks:
- mon: all monitoring services + traefik
- app: 3rd party applications
- socat: providesaccess to Docker socket from other nodes, required for running Traefik on worker nodes

# Logs

- Logs are written to `./logs` folder

# Misc

- ssh cluster@address -i ~/.ssh/gmpw7
- apt-cache madison docker-ce

# Known Issues
- When nodes are bihind of NAT (e.g. using external IP addresses) encrypted networks doesn't work in Ubuntu LTE 18.04 and other OSes running kernel >4.4, ref. https://github.com/moby/moby/issues/37115 for details
