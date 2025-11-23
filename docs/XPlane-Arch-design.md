
3 个核心部件 —— gitops-sync / gtm-controller / autoscaler —— 做成一套 正式完整的技术方案（声明式、自动化、Cloud-Neutral、可扩展）。

全部围绕一句话展开：

Git 只存“宣言”（声明），控制平面自动推导“世界应该怎样运行”，CI 执行真正的资源变更。

你得到的是一个高度自洽、可落地、可扩展、能跑真实生产的 Cloud-Neutral 控制面模型。

🔥 总览架构：声明式 → 推导 → 下发 → 执行
Git Repo (platform-config/**)
     │         ▲
     ▼         │ (写入声明)
gitops-sync ───┘
     │
     ▼ (推导实际状态)
gtm-controller  +  autoscaler
     │             │
     │ DNS API     │ Git commit
     ▼             ▼
Cloud DNS      CI / GitHub Actions
                  │
                  ▼
              Terraform / Ansible
                  │
                  ▼
        实际云资源（VM/节点/DNS记录）

1️⃣ gitops-sync：声明式的“真相输入层”

gitops-sync 的作用是：

从 Git pull 最新配置

解析 YAML → 写入控制面 DB（不是直接执行）

通知 gtm-controller / autoscaler：
“配置更新啦！请重新 reconcile（推导）。”

1.1 gitops-sync 负责同步哪些声明？

你提到：

至少同步初始化配置、上限、下限

拆成三类：

✔ A. GTM（流量 / GSLB）声明

platform-config/gtm/svc-plus.yaml

包含：

所有 domain

每个 domain 的 region pool 列表

每个 region 的基础权重（base weight）

每个 region 的 min_ready_nodes

failover 策略

健康检查配置（blackbox exporter）

DNS provider 配置（cloudflare/alidns/route53 等）

TTL

✔ B. Autoscale（扩缩容）声明

platform-config/autoscale/svc-plus.yaml

包含：

min_nodes / max_nodes

scale_up/down 阈值

scale_up/down 步长

CPU/RAM/QPS 观测窗口

多 Region 策略（global min/max regions）

区域调度权重（如某 Region 成本更低，优先扩容）

✔ C. Region/Node 配置（基础 pool 定义）

platform-config/regions/*.yaml

包含：

region 名称

provider（aws / alicloud / tencent / ucloud / custom）

instance_type

labels（存储/CPU/GPU 类型）

节点模式（containerd-only / k3s-worker / k3s-master）

gitops-sync 的工作流程
git pull
│
├── parse gtm/*.yaml → GTMPolicyStore
├── parse autoscale/*.yaml → AutoscalePolicyStore
└── parse regions/*.yaml → RegionStore


然后写入 DB，改变“声明世界”。

此时还没触发任何 DNS / 扩缩容行为。

2️⃣ gtm-controller：动态权重 & 动态剔除节点 & 下发 DNS

你的要求：

gtm-controller 自动计算每个资源池动态权重
下发资源池 DNS 解析和权重
动态剔除异常节点

我们做一个正式的 Desired → Current → Reconcile 模型（跟 K8s 一样）。

2.1 Desired（由 GitOps Sync 同步来的声明）

例如：

regions:
  - name: jp
    weight: 100
    min_ready_nodes: 1

  - name: sg
    weight: 10
    min_ready_nodes: 1

health:
  type: http
  interval: 5s
 dns:
  provider: cloudflare
  ttl: 30


这些是“静态宣言（声明式）”。

2.2 实际健康（由节点心跳 & Blackbox Exporter 汇总）

每个 region 的 NodeStore 会告诉我们：

ready nodes 数量

最差 RTT

错误率

blackbox 探测成功率

该 region 当前实际可用 IP 列表

形成：

type RegionHealth struct {
    ReadyNodes int
    RTTAvg     float64
    ErrorRate  float64
    Up         bool
}

2.3 gtm-controller 动态计算 Effective Weight

逻辑（可调）：

if readyNodes < min_ready_nodes:
    effectiveWeight = 0    // 整个 Region 下线
else:
    effectiveWeight = baseWeight * healthFactor


其中：

healthFactor = f(RTT, error_rate)


示例：

if r.RTTAvg > 300ms:
    healthFactor *= 0.7
if r.ErrorRate > 0.05:
    healthFactor *= 0.5


最终形成：

type EffectiveRegionState struct {
    Name       string
    IPs        []string
    Weight     int   // 动态
    Up         bool
}

2.4 diff current DNS state vs desired DNS state

从 Cloudflare/AliDNS/Route53 获取当前记录：

type CurrentDnsRecord struct {
    Region string
    IP     string
    Weight int
}


比较 desired & current，生成一个 diff：

哪些 IP 要加？

哪些 IP 要删？

哪些 Region 要把权重调 0？

哪些 Region 要恢复权重？

哪些 Region weight 要调整？

2.5 reconcile：最小变更写回 DNS provider
dnsProvider.Apply(diff)

2.6 “动态剔除异常节点”

我们把 Node 掉线视为 region 内部的 diff：

node 心跳超时

blackbox 失败

exporter 报错

error rate > 某阈值

RTT 过高

此 Node 直接从 region 的 IP 列表剔除 → 需要重新算 effective weight → 然后进入 DNS reconcile。

你不需要手动处理，gtm-controller 每一次 reconcile 都会把异常 node 自动移除。

3️⃣ autoscaler：声明式扩缩容 → 下发给 CI 执行

你提到：

autoscaler 自动下发给 CI 执行扩缩容配置，CI 来执行

这是整个架构的核心亮点：
Autoscaler 不直接“动云”，它只修改 infra/ 下的 desired 状态 → CI 再执行实际变更。

完全符合 GitOps 原则。

3.1 autoscaler 输入（来自 gitops-sync + metrics）
输入 1：声明的策略

（来自 GitOps Sync）：

regions:
  jp:
    min_nodes: 1
    max_nodes: 5
    scale_up_cpu: 0.7
    scale_down_cpu: 0.3

输入 2：运行时 metrics
type MetricsSnapshot struct {
    CpuAvg float64
    RttAvg float64
    ErrRate float64
}

输入 3：当前节点数量

从 infra/node-pool/jp.tfvars 里读取：

node_count = 2

3.2 autoscaler 输出：更新 infra/ 下的 desired state

autoscaler 计算：

desired_nodes = current + scale_step


然后：

✔ autoscaler 修改 infra/node-pool/jp.tfvars
node_count = 3

✔ 写 Git commit

commit message:

autoscale: svc-plus jp scale_out 2 -> 3 (cpu=0.82)

✔ push 到远程

CI 看到 infra/ 下文件变化，就执行：

terraform apply


→ 云上真正多出一台 VM。

新节点启动后：

自己安装 containerd / Nginx

调用 /nodes/register

join Region

被 GTM Controller 纳入健康池

被 API Gateway 动态纳入 upstream

整个链路闭合。

🔥 最终时序（自动运行）——你要的完整逻辑
场景：Region JP 负载飙升 → 自动扩容
步骤 1：Node/exporter 上报 CPU = 0.82

Metrics aggregator → node store → autoscaler

步骤 2：autoscaler 计算需要扩容

policy says:

scale_up_cpu = 0.7


80% > 70% → 扩容

新的 desired_nodes = 3

autoscaler 修改：

infra/node-pool/jp.tfvars:
    node_count = 3


commit & push。

步骤 3：CI 执行 Terraform apply

在 cloud 上创建 JP 的第三个节点 VM。

cloud-init 自动安装组件：

containerd

exporter

nginx

拉业务容器

调 register API

步骤 4：gtm-controller 看到 region 有 3 个 ready 节点

动态提升该 region 的 effective weight（或保持）

GTM 更新 DNS 记录：

增加新节点的 A/AAAA

region weight 保持稳态

步骤 5：API Gateway 收到新的 upstream 池

所有节点都能接入流量。

🧩 总结（全部满足你的要求）
✔ gitops-sync

同步初始化配置（GTM、autoscale、region）

同步上下限（min/max）

把 YAML → DB

Trigger reconciler

✔ gtm-controller

对每个资源池（region）自动计算动态权重

根据 node 健康自动剔除节点

下发 DNS（Cloudflare / AliDNS / Route53 / PowerDNS）

做 GSLB 动态调整

完全声明式与健康驱动组合

✔ autoscaler

基于策略 + metrics 计算 desired nodes

不直接扩缩容：而是修改 infra 目录

CI（Terraform / Ansible）执行资源调整

完全审计可追踪（Git 记录）

🚀 下一步如果你愿意，我可以帮你：
● 生成一个可直接使用的 Git 仓库 skeleton（带空文件）
platform-config/gtm/
platform-config/autoscale/
infra/node-pool/
config/ansible/
cmd/gitops-sync/
cmd/gtm-controller/
cmd/autoscaler/
