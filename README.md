# ai-agent-gitops

AI Agent 平台的 GitOps 仓库，托管三个 AI 应用及其 Kubernetes 部署清单与源码。

## 项目总览

| 项目 | 说明 | 部署清单 | 源码 |
|------|------|----------|------|
| MCP Agent | 基于 Model Context Protocol 的 HR 助手 | `apps/mcp-agent/` | `src/ai-gateway/`, `src/mcp-hr-server/` |
| SRE Agent | 集群故障巡检与告警 Agent | `apps/sre-agent/` | `src/sre-agent/` |
| RAG 应用 | 基于 Milvus + Ollama 的知识库问答 | `apps/rag-app/` | `src/rag-backend/`, `src/rag-ingestion/`, `src/rag-frontend/` |

## 目录结构

```
.
├── apps/                  # ArgoCD 管理的 Kubernetes 部署清单
│   ├── mysql/             # MySQL 数据库（含 HR 示例数据）
│   ├── mcp-agent/         # MCP Agent（gateway + server）
│   ├── milvus/            # Milvus 向量数据库（hostPath PV）
│   ├── sre-agent/         # SRE Agent（deployment + rbac）
│   └── rag-app/           # RAG 应用（frontend + backend）
├── src/                   # 各服务源码
│   ├── ai-gateway/        # MCP Gateway（SSE 转发）
│   ├── mcp-hr-server/     # MCP HR Server
│   ├── sre-agent/         # SRE Agent
│   ├── rag-backend/       # RAG 后端 API（Go + Gin）
│   ├── rag-ingestion/     # 知识库数据入库（Go）
│   └── rag-frontend/      # RAG 前端（Vue 3）
├── knowledge-base/        # 知识库文档（RAG 数据源）
└── demo/                  # 演示用资源
    ├── crash-app.yaml      # 故意崩溃的测试应用（演示故障检测）
    └── pod-crash-rule.yaml # Pod 崩溃告警规则（PrometheusRule）
```

## 依赖组件

- **MySQL**（`apps/mysql/`）— HR 示例数据存储
- **Milvus**（`apps/milvus/`）— 向量数据库，RAG 检索后端
- **Ollama** — 向量化模型 `nomic-embed-text` 与 LLM `qwen:7b-chat-q4_k_m`（需外部提供）
- **Prometheus Operator** — 告警规则（`demo/pod-crash-rule.yaml`）依赖

## 快速开始

各应用清单位于 `apps/` 目录，可通过 ArgoCD 或 `kubectl apply` 部署。

### 演示故障检测（SRE Agent）

```bash
# 1. 部署 Pod 崩溃告警规则
kubectl apply -f demo/pod-crash-rule.yaml

# 2. 部署一个故意崩溃的应用
kubectl apply -f demo/crash-app.yaml

# 3. SRE Agent 自动发现 CrashLoopBackOff 告警并处理
```
