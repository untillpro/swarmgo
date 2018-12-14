# Usage

Prerequestes:

- Go
- sh
- git

Steps:

- Fork swarmgo repo to `mycluster`
- git clone `mycluster`
- Run `go build swarmgo.go` to build swarmgo executable
- Edit `clusterfile.yml` and other configs
- Run `swarmgo node add <Alias1>=<IP1> <Alias2>=<IP2>`
  - All node are kept in `nodes.yml`
  - Note: won't be possible to use password anymore
  - Node will be added to `nodes` file
- Run `swarmgo docker`
  - Install docker to all nodes which do not have docker installed yet (ref. `nodes.yml`)
- Run `swarmgo swarm -m <Alias1> <Alias2>`
  - Install swarm `manager` modes
- Run `swarmgo swarm`
  - Install swarm in `worker` mode for all nodes which do not have swarm configured yet
  - At least one manager must be configured first


Services:
- mycluster.io/traefik
- mycluster.io/graphana
- mycluster.io/prometheus

# Under the Hood

Networks:
- traefik: traefik + consul
- webgateway: all services which should be available from outside, such services must have a label `traefik.enabled=true`

# Logs

- Logs are written to `./logs` folder

 