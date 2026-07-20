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
            { text: '加速乐三层解密', link: '/architecture/jsl-three-layers' },
            { text: '验证码挑战', link: '/architecture/captcha' },
            { text: '隐蔽性强化', link: '/architecture/stealth' }
          ]
        }
      ],
      '/api-cnvd-skills/': [
        {
          text: 'cnvd_skills 包',
          items: [
            { text: 'CnvdSkills', link: '/api-cnvd-skills/cnvd-skills' },
            { text: 'Config', link: '/api-cnvd-skills/config' },
            { text: 'VulDetail', link: '/api-cnvd-skills/vul-detail' },
            { text: 'VulList', link: '/api-cnvd-skills/vul-list' },
            { text: 'VulListQuery', link: '/api-cnvd-skills/vul-list-query' },
            { text: 'VulPatch', link: '/api-cnvd-skills/vul-patch' },
            { text: 'Proxy', link: '/api-cnvd-skills/proxy' },
            { text: '字段速查', link: '/api-cnvd-skills/fields-reference' },
            { text: 'WithConfig 对照', link: '/api-cnvd-skills/withconfig-variants' },
            { text: '示例', link: '/api-cnvd-skills/examples/basic-vul-list' }
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
