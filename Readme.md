A tool for monitor connectivity inside a ES cluster
====================================================

## Introduction

This exporter queries data from each ES nodes API. Then expose these data at /metrics. You need to put your full ES node list of every cluster need to be monitored inside **targets** folder for the expoter to queries data from.

### Manual query to ES node

```bash
curl -s 10.0.0.10:9200/_cluster/stats | jq ._nodes

10.0.0.10:9200
{
  "total": 10,
  "successful": 7,
  "failed": 3,
}
```

## Supported metrics

The exporter support 3 metrics at: ***{local_IP}:{listen_port}/metrics***

* **Successful connected node**: *elasticsearch_node_connectivity_successful{cluster,exported_instance}*
* **Disconnected node**: *elasticsearch_node_connectivity_failed{cluster,exported_instance}*
* **Total node of cluster**: *elasticsearch_node_connectivity_total{cluster,exported_instance}*

## How to use

### Target file

Put your list of ES nodes inside **"./targets"** folder

Target file is in json format

Example: *cluster1.json*

```json
{
    "cluster_name": "es_6_3",
    "node_list": [
        "{{IP_1}}:9200",
        "{{IP_2}}:9200",
        "{{IP_2}}:9200",
    ]
}
```

### Build command

```go
go run main.go -port 8080 -folder ./targets/ -log-file ./log.txt
```

Build it from source

```bash
env GOOS=linux GOARCH=amd64 go build -o ./bin/es_connectivity_exporter ./

# Run it
./es_connectivity_exporter -port 8080 -folder ./targets/ -log-file ./log.txt
```
