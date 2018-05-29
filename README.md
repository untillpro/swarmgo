# Usage

- Run `swarmgo init` to create `./clusterfile.yml`
- Check/edit `./clusterfile.yml`
- Run `swarmgo node add <Alias1>=<IP1> <Alias2>=<IP2>`
  - Note: won't be possible to use password anymore
  - Node will be added to `nodes` file
  - Now it is possible to run `swarmgo ssh <Alias> <Command>`
- Run `swarmgo docker Alias1 Alias2`
  - Install docker to all nodes which do not have docker installed yet
- Run `swarmgo docker <IP>`
  - Install docker to particular node
- Run `swarmgo swarm`
  - Install docker to all nodes

# Logs

- Logs are written to `./logs` folder