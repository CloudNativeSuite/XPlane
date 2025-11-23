Cloud-Neutral 控制平面：架构记录版（可直接长期维护）

1. 总体目标（一句话版）

构建一个 声明式、可 GitOps、可自愈、可扩缩容、Cloud-Neutral 的控制平面，实现：

全局流量调度（GTM）

动态 DNS 权重控制

Region / Node 健康治理

自动扩缩容

多 Region 弹性扩张

节点可 containerd-only 起步

不依赖任何云厂商

全流程可审计（Git 作为唯一真相源）

最小运行形态：1 控制平面 + 1 Region + 1 Node（containerd-only）即可上线业务。

2. 三大组件（声明式控制核心）
📌 2.1 GitOps Sync（输入层：声明 → 真相）

职责：

定期 git pull

解析 platform-config/**：

gtm/

autoscale/

regions/

写入控制平面数据库（GtmPolicyStore / AutoscalePolicyStore / RegionStore）

通知下游 Reconciler（gtm-controller / autoscaler）

关键词：
Git 是声明源；控制平面不直接看 YAML，而看 DB 里的声明。

📌 2.2 GTM Controller（流量 & 健康 & GSLB）

职责：

读取声明（domain/region/weight/min_ready_nodes/TTL）

收集实际健康（node 心跳、blackbox、exporter、RTT/err）

动态计算每个 region 的 effective weight

动态剔除 unhealthy 节点

调 Cloudflare / AliDNS / Route53 / PowerDNS API

执行 DNS 记录的 差异更新（reconcile）

核心机制：

Desired (来自 Git)
Status (来自 nodes/metrics)
  ↓
Effective desired
  ↓
DNS Reconcile (apply only diff)


你得到一个 Cloudflare 版的 “小型 Global Load Balancer”。

📌 2.3 Autoscaler（资源调度 & GitOps 驱动）

职责：

读取声明（min/max/scale_up/down）

收集 metrics（CPU / RTT / error rate）

推导出每个 region 的 desired_node_count

修改 infra/node-pool/<region>.tfvars

commit & push 到 Git

触发 CI / Terraform 真实创建或销毁节点

重点：
autoscaler 不直接扩容，它只改 Git（声明），CI 来执行。

这完全就是 K8s Operator 思想：
Desired → Reconcile → Provider 执行。

3. 资源池模型（Region + Node）
Region

name（jp/sg/us）

provider（aws/alicloud/tencent/ucloud/edge…）

min_nodes / max_nodes

base_weight

state（up / down / degraded）

Node

region

ip

mode：containerd-only / k3s-worker

status：up / down / drain

heartbeat（10–30s）

metrics（CPU / RTT / error）

一个 Region 最少只需要 1 台 containerd-only Node。

4. Node（containerd-only）模型

一个 Node 必须具有：

containerd / nerdctl

Nginx（本地蓝绿切换）

Exporter：node_exporter / blackbox_exporter

蓝绿部署脚本（blue/green）

自注册 API（/nodes/register）

心跳 API（/nodes/heartbeat）

蓝绿机制：

svc-blue   (port 18080)
svc-green  (port 28080)

Nginx upstream 切换 blue <-> green


单节点即可零停机发布。

5. Git 仓库结构（声明式+执行分离）
cloud-neutral-control-plane/
├── platform-config/
│   ├── gtm/
│   ├── autoscale/
│   └── regions/
│
├── infra/              # 实际资源声明
│   └── node-pool/
│       ├── jp.tfvars
│       └── sg.tfvars
│
├── config/             # Ansible 配置节点
│   ├── roles/
│   └── playbooks/
│
└── .github/workflows/  # CI 执行 Terraform


原则：
platform-config = “意图（Intent）”
infra = “资源（Desired State）”
config = “节点配置（Stateful）”

6. 核心流程
🚀 6.1 Bootstrap（最小上线）

Terraform apply → 控制平面 VM

控制平面启动：gitops-sync / gtm-controller / autoscaler

Terraform apply → Region 第一个 Node

Node → register → ready

GTM Controller → DNS = JP Node IP
服务上线

📈 6.2 自动扩容

Node metrics：CPU 0.82

autoscaler 触发 scale-out → desired_nodes = 2

修改 infra/node-pool/jp.tfvars

commit & push

CI → Terraform apply

新 Node register → 加入 pool

GTM Controller → Region weight/健康重新计算

📉 6.3 自动缩容

与扩容反向：

autoscaler 标记节点 drain

从 upstream 移除

Terraform destroy 节点

Region 回到 min_nodes

🛡 6.4 节点故障自愈

心跳丢 / blackbox 失败 → node=down

gtm-controller 立刻剔除

autoscaler 发现“节点低于 min_nodes” → 自动创建替补

🌏 6.5 Region 级故障 & GSLB 切换

region unhealthy → effective weight=0

gtm-controller → DNS 切流量到其他 region

autoscaler → 在其他 region 拉起节点（若开启 failover）

✨ 7. 体系特点（关键句式，写在文档里最漂亮）

Git = 唯一真相源（Single Source of Truth）

GTM Controller = 动态 DNS / GSLB 的大脑

Autoscaler = 用 Git 改 Terraform 不用 API 改机器

CI = 执行者，负责落地真实资源

Node = 容器工作单元，containerd-only 即可工作

Region = 弹性池，最小只需要 1台 Node

健康驱动流量，声明驱动架构

一句话总结：

我用 Git 来描述世界，用 Go 来推导世界，用 CI 来改变世界。
