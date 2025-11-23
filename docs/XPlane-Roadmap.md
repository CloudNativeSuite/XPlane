# XPlane Roadmap（核心优先级版）

## Phase 0 — Bootstrapping（最小可运行形态）

目标：
只要你给 GitRepo 填入 gtm/、autoscale/、regions/，XPlane 就能运行完整闭环。

### 📌 0.1 最小 GitOps 声明目录（你提供的结构）

```
example/gitops-config/
├── gtm/
│   └── svc-plus.yaml
├── autoscale/
│   └── svc-plus.yaml
└── regions/
    ├── cn.yaml
    └── global.yaml
```

### 📌 0.2 GitOps Sync Service（核心入口）

定期 git pull
解析 3 类声明（gtm/autoscale/regions）
写入 XPlane 内部数据库（SQLite/PG）
发布事件：OnGtmPolicyChanged、OnAutoscaleChanged、OnRegionConfigChanged
核心任务：让 XPlane “读懂” 世界。

## Phase 1 — Global Traffic Management（GTM Controller）

目标：
XPlane 具备“全球流量调度能力”，能对 DNS 作出正确的权重和健康判断。

📌 1.1 Region Model

基础权重（base_weight）

健康阈值

最小 ready 节点

当前有效节点数（auto counted）

📌 1.2 Node Model

注册

心跳

RTT

error rate

blackbox

status: up/down/drain

📌 1.3 算法：Dynamic Effective Weight
effective_weight = region.base_weight
                 × health_ratio
                 × ready_nodes_factor
                 × latency_factor

📌 1.4 权重下发（DNS Providers）

Cloudflare

AliDNS

Route53

PowerDNS

GTM Controller = XPlane 第一颗大脑。

Phase 2 — Autoscaler（Infra Desired State Engine）

目标：
不直接创建机器，而是通过 修改 Git 中的 desired state 来触发 CI / Terraform。

📌 2.1 读取 autoscale/ 下声明

内容包括：

min_nodes / max_nodes

scale_up/down 阈值

jitter、cooldown

region 配额

📌 2.2 Node Metrics Aggregator

聚合：

CPU

RTT

error rate

QPS（可选）

📌 2.3 Desired Node Count 计算
desired = function(metrics, policy)

📌 2.4 GitOps 突变（核心）

修改：

infra/node-pool/<region>.tfvars


然后 commit:

feat(xplane-autoscaler): scale jp pool to 3 nodes


由 CI 执行：

terraform plan

terraform apply

Autoscaler = XPlane 第二颗大脑。

Phase 3 — Control Loops（Reconciler Framework）

目标：
让所有组件（GTM、Autoscale、Regions、Nodes）变成 Kubernetes 风格的 Reconcile Loop。

三类 Reconciler：
3.1 Policy Reconcile（声明 -> DB）

GitOps Sync 已完成 → XPlane 主动 reconcile

3.2 WorldState Reconcile（节点健康 -> 状态机）

节点状态：

idle → warm → active → drain → delete

3.3 Action Reconcile（触发器）

GTM → DNS

Autoscaler → Git

Region → Node 模板

Node → 心跳/注册

这个阶段 XPlane 才真正“像个云控制平面”。

Phase 4 — Region Lifecycle（区域全生命周期）
📌 4.1 Region 模板（GitOps）
regions/jp.yaml
  - provider: aws
  - instance_type: t3.small
  - min_nodes: 1

📌 4.2 terraform 目录结构自动生成
infra/
└── node-pool/
    ├── jp/
    └── sg/

📌 4.3 Region Bootstrap

第一个 node 创建

node 自动注册

流量能打进去

health OK

此时 XPlane 能自动把新 region 接入流量。

Phase 5 — Node Lifecycle（containerd-only）

目标：
最低成本、最轻运行的 workload model。

📌 5.1 Node Provision

cloud-init / ansible：

安装 containerd

nerdctl

xplane-agent（心跳/注册）

blackbox

node_exporter

蓝绿发布脚本

📌 5.2 Node 自注册
POST /nodes/register

📌 5.3 Node 心跳
POST /nodes/heartbeat

📌 5.4 Node failure & auto-recovery

自动剔除 → autoscaler 补位 → region 恢复 → gtm 调整权重

Phase 6 — High Availability（双活控制面）
📌 6.1 双控制节点（active/standby 或 raft）

sqlite → pg

raft store（选配）

📌 6.2 控制面自身健康推进

健康不佳时 → stop reconcile

健康恢复 → resume reconcile

Phase 7 — 完整 XPlane（生产级）

全局 GSLB

多云自动扩缩容

跨云 failover

Node 自动初始化

GitOps 生命周期闭环

RBAC（可接 Casdoor）

XControl UI 面板

Log/Metric/Trace 对接 XScopeHub

XPlane 变成真正的 Cloud-Neutral 控制平面核心。

🔥 最终，你的核心 Deliverables（优先级 1～3）
Priority 1（能跑）

GitOps Sync

region/node 模型

node register + heartbeat

GTM Controller

DNS Provider（cloudflare）

Autoscaler（GitOps 变更）

Priority 2（能伸缩）

multi-region

weights algorithm

region failover

terraform node pool

CI 流程模板

Priority 3（能自愈）

drain / delete

node lifecycle

region lifecycle

control loop framework

实时观测（exporter 集成）
