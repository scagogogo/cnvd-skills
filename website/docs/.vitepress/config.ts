// website/docs/.vitepress/config.ts
import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'cnvd-skills',
  description: 'CNVD 漏洞信息抓取 CLI 与 Go SDK',
  base: '/cnvd-skills/',
  cleanUrls: true,
  lastUpdated: true,
  themeConfig: {
    nav: [
      { text: '指南', link: '/guide/getting-started' },
      { text: '架构', link: '/architecture/overview' },
      { text: 'cnvd_skills API', link: '/api-cnvd-skills/cnvd-skills' },
      { text: 'go-jsl API', link: '/api-gojsl/jsl-client' },
      { text: 'GitHub', link: 'https://github.com/scagogogo/cnvd-skills' }
    ],
    sidebar: {
      '/guide/': [
        {
          text: '入门',
          items: [
            { text: '快速开始', link: '/guide/getting-started' },
            { text: '安装', link: '/guide/installation' },
            { text: '配置', link: '/guide/config' }
          ]
        },
        {
          text: '使用',
          items: [
            { text: '漏洞列表抓取', link: '/guide/vul-list' },
            { text: '漏洞详情', link: '/guide/vul-detail' },
            { text: '厂商补丁', link: '/guide/vul-patch' },
            { text: '列表检索', link: '/guide/vul-list-query' },
            { text: '代理与重试', link: '/guide/proxy-retry' }
          ]
        },
        {
          text: '进阶',
          items: [
            { text: 'CLI 快速运行', link: '/guide/quickstart-cli' },
            { text: '输出格式', link: '/guide/output-format' },
            { text: '去重机制', link: '/guide/dedup' },
            { text: '节奏抖动', link: '/guide/jitter' },
            { text: '验证码识别器', link: '/guide/captcha-solver-guide' },
            { text: '并发安全', link: '/guide/concurrency' },
            { text: '问题排查', link: '/guide/troubleshooting' }
          ]
        }
      ],
      '/architecture/': [
        {
          text: '架构设计',
          items: [
            { text: '总览', link: '/architecture/overview' },
            { text: '模块划分', link: '/architecture/modules' },
            { text: '请求全链路', link: '/architecture/request-flow' },
            { text: '加速乐三层解密', link: '/architecture/jsl-three-layers' },
            { text: '验证码挑战', link: '/architecture/captcha' },
            { text: 'cookie 生命周期', link: '/architecture/cookie-lifecycle' },
            { text: '隐蔽性强化', link: '/architecture/stealth' },
            { text: 'UA 池与 Client Hints', link: '/architecture/ua-pool' },
            { text: 'TLS 指纹决策', link: '/architecture/tls-fingerprint' },
            { text: '错误处理', link: '/architecture/error-handling' },
            { text: '并发模型', link: '/architecture/concurrency-model' },
            { text: '设计取舍', link: '/architecture/design-decisions' }
          ]
        }
      ],
      '/api-cnvd-skills/': [
        {
          text: '概览',
          items: [
            { text: 'CnvdSkills', link: '/api-cnvd-skills/cnvd-skills' },
            { text: 'Config', link: '/api-cnvd-skills/config' },
            { text: '字段速查', link: '/api-cnvd-skills/fields-reference' },
            { text: 'WithConfig 对照', link: '/api-cnvd-skills/withconfig-variants' }
          ]
        },
        {
          text: '类型详解',
          collapsed: true,
          items: [
            { text: 'VulDetail', link: '/api-cnvd-skills/vul-detail' },
            { text: 'VulList', link: '/api-cnvd-skills/vul-list' },
            { text: 'VulListQuery', link: '/api-cnvd-skills/vul-list-query' },
            { text: 'VulPatch', link: '/api-cnvd-skills/vul-patch' },
            { text: 'Proxy', link: '/api-cnvd-skills/proxy' }
          ]
        },
        {
          text: '字段逐项',
          collapsed: true,
          items: [
            { text: 'VulDetail 字段', link: '/api-cnvd-skills/types/vul-detail-fields' },
            { text: 'Config 字段', link: '/api-cnvd-skills/types/config-output' },
            { text: 'VulListQuery 字段', link: '/api-cnvd-skills/types/vul-list-query-fields' },
            { text: 'VulList 字段', link: '/api-cnvd-skills/types/vul-list-fields' },
            { text: 'VulPatch 字段', link: '/api-cnvd-skills/types/vul-patch-fields' },
            { text: 'HazardLevel 字段', link: '/api-cnvd-skills/types/hazard-level-fields' },
            { text: 'VendorPatch 字段', link: '/api-cnvd-skills/types/vendor-patch-fields' },
            { text: 'ProxyResponse 字段', link: '/api-cnvd-skills/types/proxy-response-fields' }
          ]
        },
        {
          text: '方法参考',
          collapsed: true,
          items: [
            { text: 'NewCnvdSkills', link: '/api-cnvd-skills/methods/new-cnvd-skills' },
            { text: 'VulList 主流程', link: '/api-cnvd-skills/methods/vul-list-method' },
            { text: 'VulListWithQuery', link: '/api-cnvd-skills/methods/vul-list-with-query-method' },
            { text: 'RequestVulDetail', link: '/api-cnvd-skills/methods/request-vul-detail' },
            { text: 'FetchVulDetail', link: '/api-cnvd-skills/methods/fetch-vul-detail' },
            { text: 'RequestVulListByOffset', link: '/api-cnvd-skills/methods/request-vul-list-offset' },
            { text: 'RequestVulListByQuery', link: '/api-cnvd-skills/methods/request-vul-list-query' },
            { text: 'RequestVulPatch', link: '/api-cnvd-skills/methods/request-vul-patch' },
            { text: 'ParseVulDetail', link: '/api-cnvd-skills/methods/parse-vul-detail' },
            { text: 'ParseVulList', link: '/api-cnvd-skills/methods/parse-vul-list' },
            { text: 'ParseVulPatch', link: '/api-cnvd-skills/methods/parse-vul-patch' },
            { text: 'FixedProxyProvider', link: '/api-cnvd-skills/methods/fixed-proxy-provider' },
            { text: 'PinYiProxyProvider', link: '/api-cnvd-skills/methods/pinyi-proxy-provider' },
            { text: 'DefaultConfig', link: '/api-cnvd-skills/methods/default-config' },
            { text: 'WithConfig 模式总览', link: '/api-cnvd-skills/methods/withconfig-overview' }
          ]
        },
        {
          text: '示例集',
          collapsed: true,
          items: [
            { text: '基础列表抓取', link: '/api-cnvd-skills/examples/basic-vul-list' },
            { text: '单条详情', link: '/api-cnvd-skills/examples/single-detail' },
            { text: '关键词检索', link: '/api-cnvd-skills/examples/search-by-keyword' },
            { text: '日期范围', link: '/api-cnvd-skills/examples/date-range' },
            { text: '补丁抓取', link: '/api-cnvd-skills/examples/patch-fetch' },
            { text: '代理轮换', link: '/api-cnvd-skills/examples/proxy-rotation' },
            { text: '去重续抓', link: '/api-cnvd-skills/examples/dedup-resume' },
            { text: '并发抓取', link: '/api-cnvd-skills/examples/concurrent-fetch' },
            { text: 'CLI 封装', link: '/api-cnvd-skills/examples/cli-wrapper' },
            { text: '离线解析本地 HTML', link: '/api-cnvd-skills/examples/parse-local-html' }
          ]
        }
      ],
      '/api-gojsl/': [
        {
          text: 'go-jsl 包 (jsl)',
          items: [
            { text: 'JslClient', link: '/api-gojsl/jsl-client' },
            { text: 'HttpClient', link: '/api-gojsl/http-client' },
            { text: 'CaptchaSolver', link: '/api-gojsl/captcha-solver' },
            { text: '错误变量', link: '/api-gojsl/errors' },
            { text: 'Solver 实现详解', link: '/api-gojsl/solver-implementations' },
            { text: '三层解密深度解析', link: '/api-gojsl/three-layers-deep-dive' },
            { text: '示例', link: '/api-gojsl/examples/basic-get' }
          ]
        }
      ]
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/scagogogo/cnvd-skills' }
    ],
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2026 scagogogo'
    }
  }
})
