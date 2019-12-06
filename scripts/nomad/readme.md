installation:
0. `./swarmgo consul -s node1 node2 node3`
1. `./swarmgo nomad -s node1 node2 node3`

on leader node:
0. `nomad run ./traefik.job`
1. `nomad run ./myapp.job`

links:
- app frontend: http://192.168.98.10:8000/static/
- Traefik dashboard: http://192.168.98.10:8080/dashboard/

ports:
- 8000: http