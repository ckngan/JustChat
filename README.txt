Usage:

Run loadbalancer first:
go run loadbalancer.go [client ip:port1] [server ip:port2] [heartbeat ip:port3]
- [client ip:port1] address on which loadbalancer receives new client connections
- [server ip:port2] address on which loadbalancer receives new server connections
- [heartbeat ip:port3] address on which loadbalancer checks for server aliveness

Then server:
go run server.go [loadbalancer ip:port1] [udp_ping ip:port2]
- [loadbalancer ip:port1] the address of a loadbalancer
- [udp_ping ip:port2] the UDP address on which the server pings other servers for aliveness

Then clients:
go run client.go [loadbalancer ip:port1] [loadbalancer ip:port2] [loadbalancer ip:port3]
- all ip:port arguments are addresses of the three loadbalancers in the system the client can connect to
