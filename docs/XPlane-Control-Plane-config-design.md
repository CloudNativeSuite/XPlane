
0. 总览：从声明到执行的一条龙

核心口号：人类只改 Git，机器干脏活累活。

数据流回顾：

声明式 → 推导 → 下发 → 执行

Git Repo (platform-config/**, infra/**, config/**)
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

1️⃣ Git 仓库声明结构（你动 Git，控制面动世界）

目标：把“控制面想什么”和“执行层做什么”拆开，所有“想什么”都在 Git 里声明，控制面只负责对齐现实。

1.1 目录结构

建议标准化一个 mono repo（或 infra org），里头最关键三块：

platform-config/      # 业务/平台级声明（GTM、Autoscale、未来 Identity 等）
  ├── gtm/
  │   └── svc-plus.yaml
  └── autoscale/
      └── svc-plus.yaml

infra/                # 真实资源的 IaC 层（Terraform / Pulumi / Ansible inventory）
  ├── node-pool/
  │   ├── jp.tfvars
  │   └── sg.tfvars
  └── modules/...

config/               # 节点内/服务内配置，纯人类/CI 维护
  ├── svc-plus/
  └── ...


platform-config/**：控制策略声明（GTM / Autoscale）

infra/**：真实资源目标状态（节点数、规格、VPC 等）

config/**：应用配置（不要让 autoscaler / gtm 直接摸）

1.2 GTM 策略（platform-config/gtm/*.yaml）

示例：platform-config/gtm/svc-plus.yaml

service: svc-plus
domain: api.svc.plus

regions:
  - name: jp
    weight: 100
    min_ready_nodes: 1
    fallback: true
  - name: sg
    weight: 10
    min_ready_nodes: 1
    fallback: true

health:
  type: http
  path: /healthz
  interval: 5s
  timeout: 2s

dns:
  provider: cloudflare   # 也可以是 route53 / alidns / powerdns
  ttl: 30


含义：

这是“理想的 GSLB 策略”：区域、基础权重、最小可用节点数、健康检查方式。

GTM Controller 的目标：让 DNS Provider 的实际记录 + 动态权重 收敛到这个声明。

1.3 自动扩缩容策略（platform-config/autoscale/*.yaml）

示例：platform-config/autoscale/svc-plus.yaml

service: svc-plus

global:
  cpu_window: 60s
  min_regions: 1
  max_regions: 3

regions:
  jp:
    min_nodes: 1
    max_nodes: 5
    scale_up_cpu: 0.7       # 60 秒内平均 CPU > 70% 就扩容
    scale_down_cpu: 0.3
    scale_up_step: 1
    scale_down_step: 1

  sg:
    min_nodes: 0
    max_nodes: 5
    scale_up_cpu: 0.6
    scale_down_cpu: 0.25
    scale_up_step: 1
    scale_down_step: 1


含义：

这是“理想的区域节点分布+扩缩容策略”。

Autoscaler 做的事：根据 metrics + 该策略 计算每个 region 的 desired_nodes，再去改 infra 声明。

1.4 节点池声明（infra/node-pool/*.tfvars）

示例：infra/node-pool/jp.tfvars

region         = "ap-northeast-1"
service        = "svc-plus"
node_count     = 2  # 👈 Autoscaler 改的是这个
instance_type  = "t3.small"


这是 Terraform/Pulumi 层面的“目标节点数”。

Autoscaler 不直接调云 API，而是改 tfvars → 提交 Git → CI 跑 Terraform apply。

1.5 config/ 约束

规则：Autoscaler / GTM 不直接写 config/**

config/** 只由人类 / CI 管，确保“配置变更 = 人类审阅过的 PR”。

2️⃣ GitOps Sync（Go）：YAML → 控制面“真相表”

角色：YAML 解读员 + 控制面喇叭。

职责：

定期或者 webhook 触发 git pull。

解析 platform-config/**：

gtm/*.yaml → 存入 GtmPolicyStore

autoscale/*.yaml → 存入 AutoscalePolicyStore

更新完成后，通过 Notifier 告知 Reconciler：“配置变了，快看看。”

2.1 数据结构
type GitOpsSync struct {
    RepoPath     string
    PollInterval time.Duration

    GtmStore   GtmPolicyStore        // interface，方便换 DB
    AsStore    AutoscalePolicyStore
    Notifier   Notifier              // 发布 Topic 事件
}

2.2 运行循环
func (s *GitOpsSync) Run(ctx context.Context) {
    ticker := time.NewTicker(s.PollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            s.syncOnce(ctx)
        case <-ctx.Done():
            return
        }
    }
}

2.3 单次同步逻辑
func (s *GitOpsSync) syncOnce(ctx context.Context) {
    if err := gitPull(s.RepoPath); err != nil {
        log.Printf("git pull failed: %v", err)
        return
    }

    // 解析 GTM
    gtmPolicies, err := loadGtmPolicies(filepath.Join(s.RepoPath, "platform-config/gtm"))
    if err == nil {
        s.GtmStore.UpsertAll(ctx, gtmPolicies)
        s.Notifier.Notify(TopicGTMConfigChanged)
    }

    // 解析 Autoscale
    asPolicies, err := loadAutoscalePolicies(filepath.Join(s.RepoPath, "platform-config/autoscale"))
    if err == nil {
        s.AsStore.UpsertAll(ctx, asPolicies)
        s.Notifier.Notify(TopicAutoscaleConfigChanged)
    }
}


注意：
GitOps Sync 不做 DNS、不做扩缩容、不管云资源。只做两件事：

把 YAML 解析进 DB

摇铃告诉别人“声明变更了”

3️⃣ GTM Controller（Go）：声明式 GSLB / 健康检查 / DNS 对齐

角色：把 GSLB 理想世界映射到 DNS 真实世界。

3.1 输入

来自 GtmPolicyStore 的声明：

domain

regions[]（基础权重 weight，min_ready_nodes 等）

health & dns 配置

来自 NodeStore & MetricsAgg 的运行时状态：

每个 region 的 ready 节点数

RTT / error rate 等健康指标

来自 DNS Provider 的当前记录：

Cloudflare / Route53 / AliDNS / PowerDNS

3.2 计算逻辑：RegionEffective
type RegionEffective struct {
    Name            string
    BaseWeight      int
    HealthFactor    float64 // 0~1
    EffectiveWeight int
    Up              bool
}


伪逻辑：

func computeRegionEffective(p RegionPolicy, readyNodes int, hf float64) RegionEffective {
    if readyNodes < p.MinReadyNodes {
        return RegionEffective{
            Name:            p.Name,
            BaseWeight:      p.Weight,
            HealthFactor:    0,
            EffectiveWeight: 0,
            Up:              false,
        }
    }

    ew := int(float64(p.Weight) * hf)
    if ew < 0 {
        ew = 0
    }

    return RegionEffective{
        Name:            p.Name,
        BaseWeight:      p.Weight,
        HealthFactor:    hf,
        EffectiveWeight: ew,
        Up:              true,
    }
}

3.3 当前 DNS 状态

抽象一层 Provider：

type DnsRecord struct {
    Region string
    IP     string
    Weight int
}

type DnsProvider interface {
    FetchCurrentState(ctx context.Context, domain string) ([]DnsRecord, error)
    ApplyChanges(ctx context.Context, domain string, diff DnsDiff) error
}

3.4 Reconcile 循环
func (c *GtmController) reconcileService(ctx context.Context, svc string) {
    policy, err := c.store.GetGtmPolicy(ctx, svc)
    if err != nil { return }

    nodes   := c.nodeStore.ListNodesByService(ctx, svc)
    metrics := c.metricsAgg.Aggregate(nodes)

    desired := computeDesiredDnsState(policy, nodes, metrics)
    current, err := c.dnsProvider.FetchCurrentState(ctx, policy.Domain)
    if err != nil { return }

    diff := diffDnsState(desired, current)
    if diff.Empty() {
        return
    }
    if err := c.dnsProvider.ApplyChanges(ctx, policy.Domain, diff); err != nil {
        log.Printf("apply dns diff failed: %v", err)
    }
}

3.5 触发方式

配置变更：GitOps Sync 通知 TopicGTMConfigChanged

健康/metrics 周期性更新：例如每 10 秒，轮询所有 service 做一次 reconcile

模式完全是 Kubernetes 风格：Spec + Status → Desired State → Reconcile。

4️⃣ Autoscaler（Go）：声明式扩缩容 → GitOps 改 infra/

角色：把 metrics 焦虑翻译成 Git commit。

4.1 输入

来自 AutoscalePolicyStore 的策略：

每个 service/global 的 CPU 窗口、区域 min/max_nodes、阈值等

来自 NodeStore / MetricsStore 的指标：

区域级 CPU 载荷 / request rate / error rate

来自 InfraStateStore 的当前节点目标数：

解析 infra/node-pool/*.tfvars 当前 node_count

4.2 计算 desired_nodes
type RegionScaleDecision struct {
    Region        string
    CurrentNodes  int
    DesiredNodes  int
    Reason        string
}


伪逻辑：

func computeDesiredNodes(policy RegionPolicy, cpu float64, current int) RegionScaleDecision {
    desired := current
    reason  := "no-op"

    if cpu > policy.ScaleUpCPU && current < policy.MaxNodes {
        step    := policy.ScaleUpStep
        desired = min(current+step, policy.MaxNodes)
        reason  = fmt.Sprintf("scale up: cpu=%.2f", cpu)
    }

    if cpu < policy.ScaleDownCPU && current > policy.MinNodes {
        step    := policy.ScaleDownStep
        desired = max(current-step, policy.MinNodes)
        reason  = fmt.Sprintf("scale down: cpu=%.2f", cpu)
    }

    return RegionScaleDecision{
        Region:       policy.Name,
        CurrentNodes: current,
        DesiredNodes: desired,
        Reason:       reason,
    }
}

4.3 改 infra/ 并提交 Git

Autoscaler 不直接打云厂商 API，全部通过 GitOps：

type InfraMutator interface {
    UpdateNodeCount(ctx context.Context, region, service string, desired int) error
    CommitAndPush(ctx context.Context, msg string) error
}


Reconcile 伪代码：

func (a *Autoscaler) reconcileService(ctx context.Context, svc string) {
    policy, err := a.asStore.GetPolicy(ctx, svc)
    if err != nil { return }

    regionStats := a.metricsAgg.AggregateByRegion(svc)
    var decisions []RegionScaleDecision

    for region, stat := range regionStats {
        current := a.infraState.CurrentNodeCount(ctx, svc, region)
        rp      := policy.Regions[region]
        d       := computeDesiredNodes(rp, stat.CPU, current)
        if d.DesiredNodes != current {
            decisions = append(decisions, d)
        }
    }

    if len(decisions) == 0 {
        return
    }

    // 修改 tfvars
    for _, d := range decisions {
        _ = a.infraMutator.UpdateNodeCount(ctx, d.Region, svc, d.DesiredNodes)
    }

    msg := buildCommitMsg(svc, decisions)
    if err := a.infraMutator.CommitAndPush(ctx, msg); err != nil {
        log.Printf("infra commit failed: %v", err)
    }
}


示例 commit message：

autoscale: svc-plus jp 1→2 (cpu=0.82)，sg 保持不变


然后 GitHub Actions 监听 infra/** 变化：

触发 terraform plan/apply，真正去扩/缩节点

全程有审计历史：不喜欢这次扩容可以 git revert。

5️⃣ 进程内 Reconciler 模型 & Go 包结构

现在来抽象一层：把整个控制面当成一堆 Reconciler + Store + Provider。

5.1 Reconciler 抽象

定义一个通用接口：

type Reconciler interface {
    Name() string
    Start(ctx context.Context) error
}


对 GTM / Autoscaler 都可以采用类似模式：

每个 Reconciler 内部：

有一个事件 channel：接收 “配置变更 / metrics tick”

有自己的 rate limit / backoff

有 reconcileOnce() 方法，实现业务逻辑

更细一点：

type Event struct {
    Topic string
    Key   string // e.g. service name
}

type EventLoopReconciler struct {
    NameStr   string
    Events    <-chan Event
    Reconcile func(ctx context.Context, e Event)
}

5.2 Go 包结构建议

一个比较干净的分层：

cmd/xplane-control/
  main.go             # 入口，组装所有组件 & 启动

internal/config/
  config.go           # 控制面自身配置（DB / git repo 等）

internal/gitops/
  sync.go             # GitOpsSync 实现
  parser_gtm.go
  parser_autoscale.go

internal/gtm/
  controller.go       # GTMController
  compute.go          # 计算 desired DNS 状态
  provider_dns.go     # DnsProvider 抽象

internal/autoscale/
  controller.go       # Autoscaler
  compute.go          # desired_nodes 计算
  infra_mutator.go    # 修改 tfvars + git commit

internal/store/
  gtm_store.go        # GtmPolicyStore
  autoscale_store.go  # AutoscalePolicyStore
  node_store.go       # Node & metrics
  infra_state.go      # 当前 node_count 视图

internal/providers/
  dns_cloudflare.go
  dns_route53.go
  metrics_prometheus.go
  scm_git.go          # 用 go-git/exec git 操作 repo

internal/events/
  bus.go              # Notifier / PubSub 抽象
  topics.go           # Topic 常量

internal/reconcile/
  loop.go             # 简单事件循环封装

pkg/api/
  types.go            # 对外 API 所需的 shared types（可选）


特点：

internal/** 全都只在本项目内用，方便重构。

Provider（云商、DNS、Git）全部用 interface 封装，方便日后插不同 backend。

业务逻辑（gtm/autoscale）和 IO（dns/git/metrics）解耦，测试写起来轻松。

6️⃣ “按照策略和声明自动触发”的时序

给你两段时序：GTM 变更 → DNS 收敛 和 负载上升 → Autoscale → Terraform 扩容。

6.1 GTM 策略改动 → DNS 更新

用 Mermaid 描一下（可以直接丢进 docs）：

sequenceDiagram
    participant Dev as Developer
    participant Git as Git Repo
    participant Sync as GitOpsSync
    participant Store as GtmPolicyStore
    participant GTM as GtmController
    participant DNS as DNS Provider

    Dev->>Git: 修改 platform-config/gtm/svc-plus.yaml\n(提交 + push)
    loop poll
        Sync->>Git: git pull
        Git-->>Sync: 新版本
        Sync->>Sync: 解析 gtm/*.yaml
        Sync->>Store: Upsert svc-plus 策略
        Sync->>GTM: Notify(TopicGTMConfigChanged)
    end

    par 定时/事件触发
        GTM->>Store: GetPolicy("svc-plus")
        GTM->>Store: ListNodesByService("svc-plus")
        GTM->>DNS: FetchCurrentState(api.svc.plus)
        GTM->>GTM: 计算 desired DNS 状态
        GTM->>DNS: ApplyChanges(diff)
    end


解释：

人类 PR 合并后，控制面自动跟进。
GTM 控制器可按事件（配置变）+ 定时 tick 双重触发，既能快速响应，又能兜底纠偏。

6.2 负载上升 → Autoscaler → GitOps → Terraform
sequenceDiagram
    participant Nodes as Nodes & Metrics
    participant Metrics as MetricsStore
    participant Auto as Autoscaler
    participant Git as Git Repo (infra/)
    participant CI as CI / GitHub Actions
    participant TF as Terraform
    participant Cloud as Cloud Provider

    loop 每 30 秒
        Nodes->>Metrics: 上报 CPU/Metrics
        Auto->>Metrics: AggregateByRegion(svc-plus)
        Auto->>Auto: 计算 desired_nodes
        Auto->>Git: 修改 infra/node-pool/jp.tfvars\n(node_count: 1→2)
        Auto->>Git: git commit -m "autoscale: svc-plus jp 1→2"
        Auto->>Git: git push
    end

    Git-->>CI: infra/** 有新 commit
    CI->>TF: terraform plan
    CI->>TF: terraform apply
    TF->>Cloud: Create/Update VMs
    Cloud-->>Nodes: 新节点 Ready
    Nodes->>Metrics: 报告新的 CPU/节点数
    Auto->>Auto: 发现 CPU 降到阈值以下，下次可能 scale down


特点：

Autoscaler 自己不感知云 API，只管动 Git。
所有变更都有审计，想禁用自动扩容：关掉 Autoscaler；想回滚：revert commit。
小结：这就是一套“会写 Terraform 的 GTM + Autoscaler”

现在这套设计基本闭环了：

Git 仓库声明结构：清晰地分成 platform-config / infra / config。
GitOps Sync：负责把 YAML 推到控制面 DB，并发事件。
GTM Controller：根据声明 + 运行时健康，对齐 DNS 实际状态。
Autoscaler：根据声明 + metrics，推导 desired_nodes，用 GitOps 驱动 IaC 层扩缩容。
Reconciler 模型 & 包结构：统一成事件驱动的 Reconcile Loop，生产级可维护。

时序：从“改 YAML”到“云上多了台 VM”的全过程都能画出来、查得到。
