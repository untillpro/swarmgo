server {
  enabled = true
  bootstrap_expect = 3
  server_join {
    retry_join = ["{{.LeaderHost}}:4648"]
  }
}

bind_addr = "{{.Host}}"

advertise {
  rpc="{{.Host}}"
  serf="{{.Host}}"
}

consul {
  address = "127.0.0.1:8500"

  server_service_name = "nomad"
  client_service_name = "nomad-client"

  auto_advertise = true

  server_auto_join = true
  client_auto_join = true
}
