# S3-SODA-Contexture — AI Infrastructure & Observability Agent

This project implements an **AI-powered infrastructure management and observability system** that combines **S3/MinIO storage intelligence** with **Prometheus-based Kubernetes monitoring** through the **Model Context Protocol (MCP)**.

It exposes infrastructure tools (S3 bucket management, data landscape analysis, CPU anomalies, crashloop detection) as callable APIs that an **AI assistant** can query using **natural language** — powered by the **SODA Contexture** open context engine.

---

## 🚀 Features

- 🗄️ **S3/MinIO Storage Intelligence** — Bucket management, object analysis, and automatic schema inference
- 🌐 Supports **multiple Prometheus instances** (multi-cluster setup)
- 🤖 Integrated with **Ollama LLMs** (e.g., `qwen2.5-coder:7b`) and **Google Gemini**
- ⚙️ Built on **FastMCP** (Python) and **mcp-go** (Go) for tool registration and invocation
- 🧠 **SODA Contexture Engine** — Open Context Specification (OCS) for enriched AI reasoning
- 📊 Provides **17+ MCP tools** for:
  - S3 bucket creation, listing, deletion, and object inspection
  - Data landscape scanning and schema detection
  - Pod and node metric summaries
  - CrashLoop detection
  - Disk pressure alerts
  - CPU/memory anomaly detection (z-score)
  - Correlated metric analysis (Pearson correlation)
  - Event timelines and restart trend detection
  - Namespace resource summaries
  - Cluster health descriptions
- 🖥️ **Web Dashboard** — React UI with execution plans, live logs, and infrastructure state
- 🔁 **ReAct Recovery Loop** — Automatic retry and recovery for failed operations
- 🧩 Ready for integration with any monitoring chatbot or SRE copilot

---

## ⚙️ Prerequisites

You'll need the following installed:

| Requirement | Version | Purpose |
|-------------|---------|---------|
| **Go** | 1.24+ | Build and run the Go agent and MCP server |
| **Python** | 3.9+ | Run the Prometheus MCP server and client |
| **MinIO** | Latest | Local S3-compatible storage backend |
| **Minikube** | Latest | Running Kubernetes clusters (for Prometheus) |
| **Prometheus** | Latest | Deployed on each Kubernetes cluster |
| **Ollama** | Latest | Local LLM inference |
| **FastMCP** | 0.2.0+ | Python MCP server framework |
| **MongoDB** | Latest | Topology storage (for OCS engine) |

---

### Practical Minimal Setup

If you're running Ollama with a 7B model and FastMCP on the same machine:

#### ✅ CPU-Only Setup
- **CPU:** 8 cores (Intel i7 / AMD Ryzen 7 or better)
- **RAM:** 16 GB
- **Storage:** SSD (10+ GB free for model files)
- **OS:** Ubuntu 22.04+ / macOS / WSL2 on Windows
- **Performance:** Each query takes ~5–15 seconds depending on model size

#### ⚡ GPU-Accelerated Setup (Recommended)
- **GPU:** NVIDIA RTX 3060 (12 GB VRAM) or better
- **CPU:** 6+ cores
- **RAM:** 16 GB
- **Speed:** 5×–10× faster responses from Ollama

---

## 🏗️ Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        Web Dashboard (:8080)                     │
│                     React UI + WebSocket                         │
└───────────────────────────┬──────────────────────────────────────┘
                            │
                            ▼
┌──────────────────────────────────────────────────────────────────┐
│                     Go Agent Core                                │
│         Planning → Execution → ReAct Recovery                    │
│                                                                  │
│  ┌─────────────────┐  ┌────────────────┐  ┌──────────────────┐  │
│  │ S3 Tools (Go)   │  │ Contexture     │  │ AWS Tools (Go)   │  │
│  │ • create-bucket │  │ • analyze-data │  │ • create-ec2     │  │
│  │ • list-buckets  │  │ • describe-ctx │  │ • create-vpc     │  │
│  │ • list-objects  │  │ • get-metadata │  │ • create-sg      │  │
│  │ • delete-bucket │  │                │  │ • create-alb     │  │
│  └────────┬────────┘  └───────┬────────┘  └────────┬─────────┘  │
│           │                   │                     │            │
│           ▼                   ▼                     ▼            │
│      MinIO (:9000)    SODA Contexture         AWS APIs          │
└──────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────┐
│                  Python MCP Server (:8001)                        │
│              FastMCP + Prometheus Clients                         │
│                                                                  │
│  17 Tools: current_metric_for_pods, workload_metrics,            │
│  top_n_pods_by_metric, pod_network_io, pods_exceeding_cpu,       │
│  pod_status_summary, recent_pod_events, node_disk_usage,         │
│  describe_cluster_health, top_disk_pressure_nodes,               │
│  pod_restart_trend, detect_pod_anomalies,                        │
│  namespace_resource_summary, detect_crashloop_pods,              │
│  correlate_metrics, pod_event_timeline, node_condition_summary   │
│                                                                  │
│           ▼                           ▼                          │
│   Prometheus (:9090)          Prometheus (:9091)                 │
│     Cluster 1                   Cluster 2                        │
└──────────────────────────────────────────────────────────────────┘
```

---

## 📁 Project Structure

```
S3-SODA-Contexture/
├── cmd/                          # CLI entry points
├── config.yaml                   # Go agent configuration
├── pkg/
│   ├── agent/                    # AI agent core (planning, execution, ReAct recovery)
│   ├── api/                      # HTTP handlers, WebSocket, server
│   ├── aws/                      # AWS SDK client + S3 operations
│   │   ├── client.go                # AWS client (EC2, VPC, ALB)
│   │   └── s3.go                    # S3 client wrapper
│   ├── contexture/               # SODA Contexture integration
│   │   ├── context_builder.go       # OCS context builder (landscape, schema inference)
│   │   └── ocs_types.go             # Open Context Specification types
│   ├── discovery/                # Infrastructure scanner
│   ├── tools/                    # MCP tool implementations (Go)
│   │   ├── factory.go               # Tool registry and factory
│   │   ├── s3_tools.go              # S3 bucket/object tools
│   │   └── contexture_tools.go      # Data landscape & schema tools
│   └── types/                    # Shared types
├── settings/                     # Resource patterns, prompt templates
├── scripts/                      # Run and install scripts
├── web/                          # React web dashboard (pre-built)
└── states/                       # Infrastructure state (auto-generated)

contexture/                       # SODA Contexture Engine (separate repo)
├── pkg/
│   ├── mcp/                      # Python MCP Server + Clients
│   │   ├── server.py                # FastMCP server (17 Prometheus tools)
│   │   ├── client.py                # Static MCP client
│   │   └── client_dynamic.py        # Dynamic NL → workflow client
│   ├── ocs/                      # Open Context Specification engine (Go)
│   │   ├── storage/s3.go            # MinIO S3 client library
│   │   └── topology/               # Istio service mesh topology
│   └── copilot/                  # Copilot (Dynamic Prompt + Ollama)
└── config/                       # YAML configs (Prometheus, Ollama, S3, MCP)
```

---

## 🧰 Configuration

### Go Agent (`config.yaml`)

```yaml
server:
  port: 3000
  host: "localhost"

aws:
  region: "us-west-2"

agent:
  provider: "gemini"              # Options: gemini, openai, anthropic, bedrock, ollama
  model: "gemini-flash-latest"
  max_tokens: 8192
  temperature: 0.0
  dry_run: false
  enable_debug: true

web:
  port: 8080
  host: "localhost"
```

### Python MCP Server (`config/`)

```yaml
# mcp_server_config.yaml
mcp_server_url: "http://localhost:8001/mcp"

# ollama_config.yaml
ollama_url: "http://localhost:11434"
ollama_model: "qwen2.5-coder:7b"

# prometheus_config.yaml
prometheus_instances:
  - name: prometheus_1
    base_url: "http://localhost:9090"
    headers: {}
    disable_ssl: false

  - name: prometheus_2
    base_url: "http://localhost:9091"
    headers: {}
    disable_ssl: false

# s3_config.yaml
endpoint: "localhost:9000"
access_key: "minioadmin"
secret_key: "minioadmin"
use_ssl: false
region: "us-east-1"
bucket_name: "ocs-bucket"
```

---

## 🚀 Setting Up Prometheus on Two Minikube Clusters

You can simulate a multi-cluster environment using two Minikube clusters:

```bash
# Create two clusters
minikube start -p minikube1
minikube start -p minikube2
```

Enable Prometheus in both clusters:

```bash
kubectl create namespace monitoring
kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml
```

Forward ports locally:

```bash
# Cluster 1
kubectl --context=minikube1 -n monitoring port-forward svc/prometheus-operated 9090:9090

# Cluster 2
kubectl --context=minikube2 -n monitoring port-forward svc/prometheus-operated 9091:9090
```

Prometheus instances are now accessible at:
- http://localhost:9090 (Cluster 1)
- http://localhost:9091 (Cluster 2)

---

## 🏃 Running the Project

### Step 1: Set Environment Variables

```bash
# AI Provider (choose one)
export GEMINI_API_KEY="your-gemini-api-key"
# export OPENAI_API_KEY="your-openai-api-key"

# MinIO / S3 Credentials
export AWS_ACCESS_KEY_ID="minioadmin"
export AWS_SECRET_ACCESS_KEY="minioadmin"
export AWS_REGION="us-west-2"
```

### Step 2: Start MinIO

```bash
MINIO_ROOT_USER=minioadmin MINIO_ROOT_PASSWORD=minioadmin \
  minio server /tmp/minio-data --console-address ":9001"
```

Verify it's running:
```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:9000/minio/health/live
# Should return: 200
```

### Step 3: Start the MCP Server (Prometheus tools)

```bash
cd contexture/pkg/mcp
fastmcp run server.py:app --transport http --port 8001
```

### Step 4: Launch the Go Agent + Web UI

```bash
./scripts/run-web-ui.sh
```

### Step 5: Access the Web UI

Open your browser to: **http://localhost:8080**

### Step 6 (Optional): Run the Dynamic Client

For direct natural language queries against the MCP server:

```bash
cd contexture/pkg/mcp
python3 client_dynamic.py
```

---

## 💬 Usage Examples

```bash
# S3 storage analysis
"Analyze the data landscape across all S3 buckets"

# Bucket operations
"List all buckets and show their sizes"

# Kubernetes observability
"Which pods are consuming the most CPU in the last 30 minutes?"

# CrashLoop detection
"Are there any pods stuck in a CrashLoopBackOff?"

# Anomaly detection
"Detect anomalous CPU usage across all pods"

# Infrastructure creation
"Create a t3.micro EC2 instance with Ubuntu 22.04"

# Correlated metrics
"Show the correlation between CPU usage and network traffic"

# Full environment
"Set up a development environment with VPC, subnets, and EC2"
```

---

## 🧪 Running Tests

Validate all MCP tools using the integration test suite:

```bash
pytest -v test_mcp_tools.py
```

This test suite:
- Iterates through all 17 MCP tools
- Calls each tool via the MCP API
- Verifies each tool returns a valid JSON response

---

## 🤝 Contributing

We welcome contributions to improve and extend **S3-SODA-Contexture**!
Whether you're fixing a bug, improving documentation, or adding a new observability tool, your help makes the project better for everyone.

---

## 🛠️ Adding a New MCP Tool

Adding a new tool lets the AI agent expose more **Prometheus-powered capabilities** to LLMs.

### Steps

1. **Define your tool function in `pkg/mcp/server.py`**

   Each tool should:
   - Use the `@app.tool()` decorator
   - Accept keyword arguments with type hints
   - Return a valid **JSON-serializable Python dictionary**
   - Include a docstring for AI agent discoverability
   - Handle exceptions gracefully

   Example:

   ```python
   @app.tool()
   def your_new_tool_name(
       metric_name: str = "container_cpu_usage_seconds_total",
       window: str = "5m"
   ) -> Dict[str, Any]:
       """
       Short description of what this tool does.
       """
       if not prometheus_clients:
           return {"error": "No Prometheus clients initialized"}

       all_results = {}
       for prom_name, client in prometheus_clients.items():
           try:
               # Step 1: Build your PromQL query
               query = f'rate({metric_name}[{window}])'

               # Step 2: Execute against Prometheus
               response = client.custom_query(query=query)

               # Step 3: Parse and structure the response
               results = []
               for item in response:
                   try:
                       value = float(item["value"][1])
                   except (KeyError, ValueError, IndexError):
                       value = None
                   results.append({
                       "pod": item.get("metric", {}).get("pod", "unknown"),
                       "value": value
                   })

               all_results[prom_name] = results
           except Exception as e:
               return {"error": str(e)}

       # Step 4: Return a JSON-serializable response
       return {
           "metric": metric_name,
           "results_per_prometheus": all_results,
           "timestamp": datetime.now().isoformat()
       }
   ```

2. **Register the Tool**

   The `@app.tool()` decorator automatically registers your tool with the FastMCP server. Just:
   1. Add your function to `server.py`
   2. Restart the MCP server
   3. Verify with the test suite:

   ```bash
   pytest -v test_mcp_tools.py
   ```

3. **Adding a Go MCP Tool (S3/Contexture)**

   For S3 or infrastructure tools, add to the Go side:

   ```go
   // In pkg/tools/your_tool.go
   func NewYourTool(client *aws.Client, actionType string, logger *logging.Logger) interfaces.MCPTool {
       return &YourTool{
           client: client,
           tool: mcp.NewTool("your-tool-name",
               mcp.WithDescription("What this tool does"),
               mcp.WithString("param_name", mcp.Description("Parameter description"), mcp.Required()),
           ),
       }
   }

   func (t *YourTool) Execute(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
       // Implementation here
   }
   ```

   Then register it in `pkg/tools/factory.go`.

---

## 🔒 Security Considerations

- **API Keys** — Never commit API keys to version control
- **MinIO Credentials** — Change default `minioadmin` credentials in production
- **AWS Permissions** — Use least-privilege IAM policies
- **Dry Run** — Always test with `dry_run: true` in `config.yaml` first
- **Network Security** — Run in private networks when possible

---

## 🛠️ Troubleshooting

<details>
<summary><strong>MinIO Connection Refused (port 9000)</strong></summary>

```bash
lsof -i :9000
MINIO_ROOT_USER=minioadmin MINIO_ROOT_PASSWORD=minioadmin \
  minio server /tmp/minio-data --console-address ":9001"
```
</details>

<details>
<summary><strong>429 Quota Exceeded (AI Provider)</strong></summary>

The smart context capping mechanism limits token usage. If you still hit limits:
```yaml
agent:
  max_tokens: 4096
```
Or switch to local Ollama:
```yaml
agent:
  provider: "ollama"
  model: "qwen2.5-coder:7b"
```
</details>

<details>
<summary><strong>Decision validation failed: confidence too low</strong></summary>

```yaml
agent:
  max_tokens: 10000
```
</details>

<details>
<summary><strong>Go Build Issues</strong></summary>

```bash
go clean -modcache && go mod download && go mod tidy && go build ./...
```
</details>

---

<div align="center">

**Built with ❤️ using SODA Contexture**

*Empowering infrastructure management through AI and enriched context*

[⭐ Star this repo](https://github.com/Venksaiabhishek/S3-SODA-Contexture) · [🐛 Report Bug](https://github.com/Venksaiabhishek/S3-SODA-Contexture/issues) · [💡 Request Feature](https://github.com/Venksaiabhishek/S3-SODA-Contexture/issues)

</div>
