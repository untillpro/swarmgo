installation:
0. `./swarmgo consul -s node1 node2 node3`
1. `./swarmgo nomad -s node1 node2 node3`

on leader node:
0. `nomad run ./traefik.job`
1. `nomad run ./myapp.job`

links:
- app frontend: http://192.168.98.10:8000/static/
- http://192.168.99.12:8500/ui
  - Accessed from outside for dev purposes only. Should be disabled in production by removing `-client "0.0.0.0` from `consul.service` file
- Traefik dashboard: http://192.168.98.10:8080/dashboard/

ports:
- 8000: http