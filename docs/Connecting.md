# Connecting

Agents connect to each other initially over TOR.

There is no authentication required to just connect.



# Agent Admin

## Grpc Web Proxy

```bash
./grpcwebproxy \
  --backend_addr=localhost:40051 \
  --server_bind_address=127.0.0.1 \
  --server_http_tls_port=4002 \
  --run_http_server=true \
  --run_tls_server=false \
  --use_websockets
```