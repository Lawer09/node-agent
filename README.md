# singbox-node-agent

轻量探测 agent，支持两类来源：
- 静态 YAML 节点
- 订阅链接拉取并自动刷新

订阅处理规则：
- 自动尝试 base64 解码
- 按行解析 URI
- 仅纳入 `vless://` 且 `security=reality` 的节点
- 自动提取 `uuid / host / port / pbk / sid / sni(servername) / fp / tag`
- 像 `ss://` 这类非 VLESS Reality 节点会被跳过

## 新增：更细粒度 handshake 分类

失败阶段现在会拆成：
- `config`
- `spawn`
- `listen`
- `socks_dial`
- `tls_clienthello`
- `reality_verify`
- `proxy_request`
- `http_response`

失败原因示例：
- `listen_timeout`
- `dns_resolve_failed`
- `tcp_connect_failed`
- `socks_connect_failed`
- `tls_handshake_timeout`
- `tls_clienthello_rejected`
- `reality_public_key_mismatch`
- `reality_short_id_mismatch`
- `reality_sni_mismatch`
- `utls_fingerprint_issue`
- `upstream_timeout`
- `http_non_2xx`

## 构建

```bash
go mod tidy
go build -o node-agent ./cmd/agent
```

## 运行

```bash
CONFIG_PATH=./configs/config.yaml ./node-agent
```

## 指标

- `/metrics`
- `/healthz`

关键指标：
- `node_probe_up{node_id,server,port,phase}`
- `node_probe_total{node_id,server,port,status,phase,error_type}`
- `node_probe_duration_seconds{node_id,server,port,phase}`
- `node_probe_last_success_timestamp_seconds{node_id,server,port}`
- `node_source_active_nodes`
- `node_source_refresh_total`

## 关键配置

```yaml
subscription:
  enabled: true
  url: "https://your-subscription.example.com/sub"
  reload_seconds: 300
```

## 订阅样例映射

原始：

```text
vless://uuid@8.8.8.8:443?...&security=reality&pbk=xxx&sid=yyy&sni=www.apple.com&fp=qq#SG-48
```

映射后：
- `uuid` -> `UUID`
- `8.8.8.8` -> `Server`
- `443` -> `ServerPort`
- `pbk` -> `PublicKey`
- `sid` -> `ShortID`
- `sni` 或 `servername` -> `ServerName`
- `fp` -> `UTLSFingerprint`
- `#SG-48` -> `Name`
