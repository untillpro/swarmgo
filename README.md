# Usage

- Run `swarmgo init` to create `./clusterfile.yml`
- Check/edit `./clusterfile.yml`
- Run `swarmgo node add <Alias1>=<IP1> <Alias2>=<IP2>`
  - All node are kept in `nodes.yml`
  - Note: won't be possible to use password anymore
  - Node will be added to `nodes` file
  - Now it is possible to run `swarmgo ssh <Alias> <Command>`
- Run `swarmgo docker`
  - Install docker to all nodes which do not have docker installed yet (ref. `nodes.yml`)
- Run `swarmgo swarm -m <Alias1> <Alias2>`
  - Install swarm `manager` modes
- Run `swarmgo swarm`
  - Install swarm in `worker` mode for all nodes which do not have swarm configured yet
  - At least one manager must be configured first

# Logs

- Logs are written to `./logs` folder

# Links

- https://jmaitrehenry.ca/2017/12/15/using-traefik-with-docker-swarm-and-consul-as-your-load-balancer/
 