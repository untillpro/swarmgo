# Usage

Prerequestes:

- Go
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
- Run `swarmgo keys` to generate new or specify existing SSH keys
  - `swarmgo keys` generates new key pair
  - `swarmgo keys -p PRIVATE_KEY_PATH -u PUBLIC_KEY_PATH` tells to use the existing key pair
  - Keys are kept in `nodes/swarmgo-config.yml` 
- Run ``eval `swarmgo agent` ``
  - Command starts `ssh-agent` enabling single sign-on in the current terminal session
- Run `swarmgo add <Alias1>=<IP1> <Alias2>=<IP2>`
  - All node are kept in `nodes.yml`
  - Note: won't be possible to use password anymore
  - Node will be added to `nodes` file
  - Use `swarmgo add -s` option when user specified as `ClusterUser` in `swarmgo-config.yml` already exists and SSH access is configured for on nodes being added. 
- Run `swarmgo docker`
  - Install docker to all nodes which do not have docker installed yet (ref. `nodes.yml`)
- Run `swarmgo swarm -m <Alias1> <Alias2>`
  - Install swarm `manager` modes
- Run `swarmgo swarm`
  - Install swarm in `worker` mode for all nodes which do not have swarm configured yet
  - At least one manager must be configured first
- Run `swarmgo traefik` to deploy traefik
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
- consul: traefik + consul
- mon: all monitoring services + traefik
- app: 3rd party applications
- socat: providesaccess to Docker socket from other nodes, required for running Traefik on worker nodes

# Logs

- Logs are written to `./logs` folder

 
# Misc

- ssh cluster@address -i ~/.ssh/gmpw7
- apt-cache madison docker-ce