---
layout: home

hero:
  name: cnvd-skills
  text: CNVD 漏洞信息抓取工具
  tagline: 自动穿透加速乐三层加密与验证码挑战，提供 CLI 与 Go SDK
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/getting-started
    - theme: alt
      text: 架构总览
      link: /architecture/overview

features:
  - title: 加速乐三层解密
    details: goja 求值 + md5/sha1/sha256 暴力匹配，自动算出 __jsl_clearance_s
  - title: 验证码自动通过
    details: 可插拔 CaptchaSolver，配合 ddddocr 全自动识别中文词组验证码
  - title: 隐蔽性强化
    details: 统一 HttpClient + cookie jar + 浏览器级 Header + UA 池 + 节奏抖动
  - title: 列表检索
    details: 按关键词/日期/厂商/级别定向检索，支持全量与定向落盘
---
