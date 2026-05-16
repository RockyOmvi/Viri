# Monitoring and Alerting

This document describes the Prometheus-based monitoring stack for Viri mainnet validators and full nodes.

## Architecture

```
+------------------+       +------------------+       +-------------------+
|  Viri Node       |       |  Viri Node       |       |  Node Exporter    |
|  /metrics :8545  |       |  /metrics :8546  |       |  :9100            |
+--------+---------+       +--------+---------+       +---------+---------+
         |                          |                           |
         +--------------------------+---------------------------+
                                    |
                         +----------+----------+
                         |     Prometheus      |
                         |  (scrape + rules)   |
                         +----------+----------+
                                    |
                    +---------------+---------------+
                    |               |               |
            +-------+-+     +------+------+   +----+------+
            | Alert-  |     |   Grafana   |   |  PagerDuty|
            | manager |     |  Dashboard  |   |  / Slack  |
            +---------+     +-------------+   +-----------+
```

### Components

| Component | Port | Purpose |
|-----------|------|---------|
| Prometheus | 9090 | Metrics aggregation and alerting rules |
| Alertmanager | 9093 | Alert routing, deduplication, silencing |
| Grafana | 3000 | Dashboard visualization |
| Node Exporter | 9100 | System metrics (CPU, RAM, disk, network) |
| Virid RPC Metrics | 8545/metrics | Chain and consensus metrics |
| Virid API Metrics | 8546/metrics | Application and HTTP metrics |

---

## 1. Prometheus Metrics

### Consensus Metrics (`consensus_*`)

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `consensus_height` | Gauge | Current block height | — |
| `consensus_view` | Gauge | Current view/round number | — |
| `consensus_phase` | Gauge | Phase: 0=idle, 1=prepare, 2=precommit, 3=commit, 4=decide | — |
| `consensus_validators` | Gauge | Number of active validators | — |
| `consensus_block_finalized_total` | Counter | Total blocks finalized | — |
| `consensus_view_changes_total` | Counter | Total view changes (timeouts) | — |
| `consensus_proposals_total` | Counter | Total proposals sent/received | — |
| `consensus_votes_total` | Counter | Total votes sent/received | — |
| `consensus_invariant_violations_total` | Counter | Detected invariant violations | — |
| `consensus_rate_limited_total` | Counter | Messages rate-limited by consensus | — |

### P2P Metrics (`p2p_*`)

| Metric | Type | Description |
|--------|------|-------------|
| `p2p_peers_connected` | Gauge | Number of connected peers |
| `p2p_bytes_in_total` | Counter | Total bytes received |
| `p2p_bytes_out_total` | Counter | Total bytes sent |
| `p2p_messages_in_total` | Counter | Total messages received |
| `p2p_messages_out_total` | Counter | Total messages sent |

### Node Metrics (`node_*`, `mempool_*`)

| Metric | Type | Description |
|--------|------|-------------|
| `node_uptime_seconds` | Gauge | Node uptime in seconds |
| `node_is_syncing` | Gauge | 1 if syncing, 0 if caught up |
| `mempool_pending_txs` | Gauge | Pending transactions in mempool |

### HTTP Metrics (`viri_http_*`)

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `viri_http_requests_total` | Counter | Total HTTP requests | server, method, status |
| `viri_http_request_duration_seconds` | Histogram | Request latency | server, method |
| `viri_http_in_flight_requests` | Gauge | Currently processing requests | server |

### Chain Metrics (`viri_chain_*`, `viri_p2p_*`, `viri_service_*`)

| Metric | Type | Description | Labels |
|--------|------|-------------|--------|
| `viri_chain_block_height` | Gauge | Current block height | server |
| `viri_p2p_peer_count` | Gauge | Current peer count | server |
| `viri_service_ready` | Gauge | 1 if ready, 0 if not | server |

---

## 2. Critical Alert Rules

All alert rules are defined in `monitoring/alerting_rules.yml`. Copy to Prometheus:

```bash
sudo mkdir -p /etc/prometheus/rules
sudo cp monitoring/alerting_rules.yml /etc/prometheus/rules/
```

### Alert Descriptions

| Alert Name | Severity | Condition | For | Description |
|------------|----------|-----------|-----|-------------|
| `ViriNodeDown` | critical | `up{job="viri-node"} == 0` | 1m | Node is completely down |
| `ViriChainStalled` | critical | `increase(viri_chain_block_height[10m]) == 0` | 10m | No new blocks produced |
| `ViriPeerDrop` | warning | `viri_p2p_peer_count < 1` | 5m | Node has no peers |
| `ViriHighErrorRate` | warning | Rate of 5xx > 10% | 5m | High HTTP error rate |
| `ViriHighInFlightRequests` | warning | In-flight > 100 | 2m | Possible DDoS or traffic spike |
| `ViriReadyStateNotReady` | warning | `viri_service_ready == 0` | 10m | Node has been not-ready too long |
| `ViriRateLimiting` | info | Rate of 429 > 10/s | 5m | Rate limiting active |

### Prometheus Configuration

```yaml
# /etc/prometheus/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "rules/viri_alerting_rules.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets:
            - localhost:9093

scrape_configs:
  - job_name: "viri-node"
    static_configs:
      - targets: ["localhost:8545"]
        labels:
          instance: "viri-validator-01"
          role: "validator"
    metrics_path: /metrics

  - job_name: "viri-node-api"
    static_configs:
      - targets: ["localhost:8546"]
        labels:
          instance: "viri-validator-01"
          role: "api"
    metrics_path: /metrics

  - job_name: "viri-nodes"
    file_sd_configs:
      - files:
          - "nodes.yml"
        refresh_interval: 5m
    metrics_path: /metrics

  - job_name: "node-exporter"
    static_configs:
      - targets: ["localhost:9100"]
```

### Multi-Node Discovery

Edit `monitoring/nodes.yml` to include all validators:

```yaml
- targets:
    - "validator-01.example.com:8545"
    - "validator-02.example.com:8545"
    - "validator-03.example.com:8545"
  labels:
    role: validator
    network: mainnet

- targets:
    - "full-node-01.example.com:8545"
  labels:
    role: full-node
    network: mainnet
```

---

## 3. Grafana Dashboard

### Import the Dashboard

```bash
# The dashboard JSON is at:
# monitoring/grafana_dashboard.json

# Import via Grafana UI:
# 1. Log into Grafana (http://localhost:3000)
# 2. Dashboards → Import
# 3. Upload grafana_dashboard.json
# 4. Select Prometheus as the data source
# 5. Click Import
```

### Dashboard Panels

| Panel | Query | Type |
|-------|-------|------|
| **Block Height** | `viri_chain_block_height{server="rpc"}` | Stat |
| **Peer Count** | `viri_p2p_peer_count{server="rpc"}` | Stat |
| **Block Production Rate** | `rate(consensus_block_finalized_total[5m])` | Time series |
| **View Changes** | `rate(consensus_view_changes_total[5m])` | Time series |
| **Active Validators** | `consensus_validators` | Stat |
| **Consensus Phase** | `consensus_phase` | State timeline |
| **Memory Usage** | `process_resident_memory_bytes` | Gauge |
| **Mempool Size** | `mempool_pending_txs` | Time series |
| **Request Latency (P99)** | `histogram_quantile(0.99, rate(viri_http_request_duration_seconds_bucket[5m]))` | Time series |
| **Error Rate** | `rate(viri_http_requests_total{status=~"5.."}[5m])` | Time series |
| **Disk Usage** | `node_filesystem_avail_bytes{mountpoint="/var/lib/viri"}` | Gauge |
| **Network I/O** | `rate(p2p_bytes_in_total[5m])` / `rate(p2p_bytes_out_total[5m])` | Time series |

---

## 4. Alerting Configuration

### PagerDuty

```yaml
# /etc/alertmanager/alertmanager.yml
route:
  group_by: ['alertname', 'instance']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: 'pagerduty-critical'
  routes:
    - match:
        severity: critical
      receiver: 'pagerduty-critical'
      repeat_interval: 15m
    - match:
        severity: warning
      receiver: 'slack-warning'

receivers:
  - name: 'pagerduty-critical'
    pagerduty_configs:
      - routing_key: <PAGERDUTY_SERVICE_KEY>
        severity: critical
        description: '{{ .GroupLabels.alertname }} - {{ .GroupLabels.instance }}'

  - name: 'slack-warning'
    slack_configs:
      - api_url: <SLACK_WEBHOOK_URL>
        channel: '#viri-alerts'
        title: '{{ .GroupLabels.alertname }}'
        text: '{{ .CommonAnnotations.description }}'
```

### Slack (Direct)

```yaml
receivers:
  - name: 'slack-alerts'
    slack_configs:
      - api_url: https://hooks.slack.com/services/<YOUR_WEBHOOK>
        channel: '#viri-alerts'
        send_resolved: true
        title: '{{ .GroupLabels.alertname }}'
        text: >-
          {{ range .Alerts }}
            *Alert:* {{ .Annotations.summary }}
            *Description:* {{ .Annotations.description }}
            *Severity:* {{ .Labels.severity }}
            *Instance:* {{ .Labels.instance }}
            {{ end }}
```

### Pushover (Individual)

```yaml
receivers:
  - name: 'pushover'
    pushover_configs:
      - user_key: <PUSHOVER_USER_KEY>
        token: <PUSHOVER_APP_TOKEN>
        title: '{{ .GroupLabels.alertname }}'
        message: '{{ .CommonAnnotations.description }}'
        priority: '{{ if eq .CommonLabels.severity "critical" }}1{{ else }}0{{ end }}'
```

---

## 5. SLO Targets

| SLO | Target | Measurement | Alert |
|-----|--------|-------------|-------|
| **Block time** | 95% of blocks within target + 1s | Block timestamp deltas | Alert if < 90% over 1h |
| **Consensus completion** | 99.9% of views complete without timeout | `consensus_view_changes_total` | Alert if > 0.1% view change rate |
| **Peer connectivity** | Each validator connected to ≥5 peers 99.9% uptime | `viri_p2p_peer_count` | Alert if < 5 peers for 5 minutes |
| **State sync** | New nodes sync within 1 hour | Time from start to sync completion | Alert if sync > 1 hour |

### Block Time Monitoring

```bash
# Monitor block time from any node
while true; do
  BLOCK=$(curl -s -X POST http://localhost:8545 \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
    | jq -r '.result')
  echo "$(date -u): Block $((BLOCK))"
  sleep 2
done
```

### Prometheus SLO Recording Rules

```yaml
# Add to alerting_rules.yml:
groups:
  - name: viri-slos
    interval: 1m
    rules:
      - record: slo:block_time_ratio
        expr: |
          rate(consensus_block_finalized_total[1h])
          / on() (1 / 1)  # target block time = 1s

      - alert: ViriBlockTimeSLO
        expr: slo:block_time_ratio < 0.9
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "Block time SLO breach"
          description: "Less than 90% of blocks produced within target time."
```

---

## 6. Uptime Monitoring for Validators

### External Uptime Checks

```bash
# Using /health endpoint (no auth required)
# Every 1 minute from an external monitoring service:

curl -s https://rpc.viri.network/health | jq .status
# Expected: "ok"

# Using /ready endpoint
curl -s https://rpc.viri.network/ready
# Expected: HTTP 200
```

### Blackbox Exporter (Prometheus)

```yaml
# Add to prometheus.yml:
scrape_configs:
  - job_name: 'blackbox'
    metrics_path: /probe
    params:
      module: [http_2xx]
    static_configs:
      - targets:
          - https://rpc.viri.network/health
          - https://api.viri.network/api/v1/health
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: localhost:9115  # Blackbox exporter
```

### Third-Party Monitoring Services

| Service | Check Type | Interval | Notes |
|---------|-----------|----------|-------|
| Better Uptime | HTTP /health | 1 min | Free tier available |
| Checkly | API check | 1 min | Good for multi-region |
| Datadog Synthetic | Browser + API | 5 min | Comprehensive |
| Grafana Cloud | Prometheus + alerts | 15s | Integrated with stack |

---

## 7. Log Aggregation

### Local Logging

Viri outputs structured JSON logs by default:

```json
{"level":"info","time":"2026-06-01T12:00:00Z","message":"Block finalized","height":12345,"hash":"0xabc..."}
{"level":"warn","time":"2026-06-01T12:00:01Z","message":"View change triggered","view":42,"reason":"timeout"}
{"level":"error","time":"2026-06-01T12:00:02Z","message":"Failed to connect to peer","peer":"16Uiu2HAm...","error":"connection refused"}
```

### Log Rotation

```bash
# The virid-backup service also handles log rotation
# Config in deploy/systemd/virid.service:
# StandardOutput=append:/var/lib/viri/logs/virid.log
# StandardError=append:/var/lib/viri/logs/virid.log

# Log rotation is handled by systemd's journald:
sudo journalctl --vacuum-time=30d
```

### Centralized Log Aggregation

#### Option A: Loki + Grafana

```bash
# Install Loki
wget https://github.com/grafana/loki/releases/download/v2.9.0/loki-linux-amd64.zip
unzip loki-linux-amd64.zip
sudo mv loki-linux-amd64 /usr/local/bin/loki

# Install Promtail (log agent)
wget https://github.com/grafana/loki/releases/download/v2.9.0/promtail-linux-amd64.zip
unzip promtail-linux-amd64.zip
sudo mv promtail-linux-amd64 /usr/local/bin/promtail

# Promtail config to ship virid logs:
# /etc/promtail/promtail.yml
scrape_configs:
  - job_name: virid
    static_configs:
      - targets: [localhost]
        labels:
          job: virid
          instance: validator-01
          __path__: /var/lib/viri/logs/*.log
```

#### Option B: Elasticsearch + Filebeat + Kibana

```bash
# Filebeat config:
filebeat.inputs:
  - type: log
    enabled: true
    paths:
      - /var/lib/viri/logs/*.log
    json.keys_under_root: true
    json.add_error_key: true

output.elasticsearch:
  hosts: ["https://elasticsearch:9200"]
  index: "viri-logs-%{+yyyy.MM.dd}"
```

#### Option C: Datadog / New Relic

```bash
# Datadog agent picks up journald logs automatically:
# /etc/datadog-agent/conf.d/journald.d/conf.yaml
logs:
  - type: journald
    source: virid
    include_units:
      - virid.service
```

---

## 8. Prometheus Setup Quickstart

```bash
# 1. Install Prometheus
wget https://github.com/prometheus/prometheus/releases/download/v2.52.0/prometheus-linux-amd64.tar.gz
tar -xzf prometheus-linux-amd64.tar.gz
sudo cp prometheus-linux-amd64/prometheus /usr/local/bin/
sudo cp prometheus-linux-amd64/promtool /usr/local/bin/

# 2. Install Node Exporter
wget https://github.com/prometheus/node_exporter/releases/download/v1.7.0/node_exporter-linux-amd64.tar.gz
tar -xzf node_exporter-linux-amd64.tar.gz
sudo cp node_exporter-linux-amd64/node_exporter /usr/local/bin/

# 3. Install Grafana
sudo apt-get install -y software-properties-common
sudo add-apt-repository "deb https://packages.grafana.com/oss/deb stable main"
sudo apt-get update
sudo apt-get install grafana

# 4. Configure
sudo cp monitoring/prometheus.yml /etc/prometheus/
sudo mkdir -p /etc/prometheus/rules
sudo cp monitoring/alerting_rules.yml /etc/prometheus/rules/
sudo cp monitoring/nodes.yml /etc/prometheus/

# 5. Start services
sudo systemctl enable prometheus
sudo systemctl enable node_exporter
sudo systemctl enable grafana-server
sudo systemctl start prometheus
sudo systemctl start node_exporter
sudo systemctl start grafana-server

# 6. Import Grafana dashboard (monitoring/grafana_dashboard.json)
```

---

## 9. Key Metrics Dashboard Snapshot

The pre-built Grafana dashboard (`monitoring/grafana_dashboard.json`) includes:

**Row 1: Chain Overview**
- Block Height (stat)
- Peer Count (stat)
- Block Production Rate (graph)
- Active Validators (stat)

**Row 2: Consensus Health**
- View Changes / Timeouts (graph)
- Consensus Phase Timeline (state timeline)
- Proposal / Vote Rate (graph)
- Consensus Errors (stat)

**Row 3: Node Health**
- CPU Usage (gauge)
- Memory Usage (gauge)
- Disk Usage (gauge)
- Uptime (stat)

**Row 4: Network**
- P2P Bytes In/Out (graph)
- Messages In/Out (graph)

**Row 5: Application**
- Request Latency (heatmap)
- Error Rate (graph)
- Mempool Size (graph)
- Rate Limiting Activity (graph)

---

## 10. Alert Testing

Before mainnet launch, verify all alerts fire correctly:

```bash
# 1. Stop the node to test ViriNodeDown
sudo systemctl stop virid
# Wait 1 minute — alert should fire
sudo systemctl start virid

# 2. Block peer connectivity to test ViriPeerDrop
sudo ufw deny 30303/tcp
# Wait 5 minutes — alert should fire
sudo ufw allow 30303/tcp

# 3. Generate high error rate for ViriHighErrorRate
# Send invalid requests:
for i in $(seq 1 100); do
  curl -s -X POST http://localhost:8545 \
    -H "Content-Type: application/json" \
    -d '{"invalid":"json"}' > /dev/null
done

# 4. Trigger rate limiting test
for i in $(seq 1 200); do
  curl -s http://localhost:8545/metrics > /dev/null &
done
```
