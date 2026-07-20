---
outline: deep
---

# 架构总览

本页给出 cnvd-skills 的整体模块关系与请求端到端时序。

## 模块关系

CLI 通过 CnvdSkills 编排业务，CnvdSkills 持有默认 JslClient，JslClient 内部组合 HttpClient 与 resty，最终访问 CNVD。

```mermaid
graph LR
    CLI --> CnvdSkills
    CnvdSkills --> JslClient
    JslClient --> HttpClient
    HttpClient --> resty
    resty --> CNVD
```

## 请求端到端时序

一次完整请求从 CLI 入口到 CNVD 响应的全过程。

```mermaid
sequenceDiagram
    participant CLI
    participant CnvdSkills
    participant JslClient
    participant HttpClient
    participant CNVD
    CLI->>CnvdSkills: 业务调用
    CnvdSkills->>JslClient: 委托请求
    JslClient->>HttpClient: 组装请求
    HttpClient->>CNVD: 发送 HTTP
    CNVD-->>HttpClient: 加速乐响应
    HttpClient-->>JslClient: 原始响应
    JslClient-->>CnvdSkills: 解密后数据
    CnvdSkills-->>CLI: 结构化结果
```

> 本页内容将在后续任务填充。
