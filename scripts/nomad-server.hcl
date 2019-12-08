server {
  enabled = true
  bootstrap_expect = 3
}

client {
  enabled = true
  network_interface = "eth1"
}

plugin "raw_exec" {
  config {
    enabled = true
  }
}

bind_addr = "{{.Host}}"

advertise {
  rpc="{{.Host}}"
  serf="{{.Host}}"
  http="{{.Host}}"  
}

consul {
  address = "127.0.0.1:8500"

  server_service_name = "nomad"
  client_service_name = "nomad-client"

  auto_advertise = true

  server_auto_join = true
  client_auto_join = true
}
