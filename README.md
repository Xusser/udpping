# UDPPing
A tool to test latency with UDP protocol

### Running
```
# Run server with 0.0.0.0:5555
./udpping_server_linux_amd64

# Run server with 127.0.0.1:1234
./udpping_server_linux_amd64 -l 127.0.0.1 -p 1234

# Run client to ping 1.1.1.1:5555
./udpping_client_linux_amd64 1.1.1.1

# Run client to ping 127.0.0.1:1234, with 500 bytes data in udp payload
./udpping_client_linux_amd64 127.0.0.1 1234 500
```