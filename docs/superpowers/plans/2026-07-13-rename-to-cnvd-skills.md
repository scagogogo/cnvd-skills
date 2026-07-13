# 仓库重命名 cnvd-crawler → cnvd-skills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: `superpowers:subagent-driven-development`
> Steps use checkbox (`- [ ]`) syntax.

**Goal:** 将 GitHub 仓库 `scagogogo/cnvd-crawler` 及本地全套命名重命名为 `scagogogo/cnvd-skills`，涵盖远端仓库名、本地文件夹、Go module 名、Go 包目录/包名、导出标识符，并保证编译与测试通过。

**Architecture:** 数据流是「命名一致性传播」：远端 GitHub 仓库名（权威源）→ `origin` remote URL 更新 → 本地 `go.mod` module 名 → Go 包目录 `cnvd_crawler/`→`cnvd_skills/` + 8 个源文件的 `package` 声明 → 导出符号 `CnvdCrawler`/`NewCnvdCrawler`→`CnvdSkills`/`NewCnvdSkills` → `main.go` 的 import 与调用 → 本地工作目录文件夹名 → 推送到远端。关键组件：`gh repo rename`（远端）、`git remote set-url`（remote）、`go.mod`（module）、`cnvd_crawler/`（包目录）、`main.go`（入口）。理由：Go 的 import 路径必须与 module 名 + 仓库 URL 一致，因此要自顶向下逐层改名，避免中间状态编译失败。

**Tech Stack:** Go 1.18, GitHub CLI (gh) 2.94.0, git, 模块 `github.com/scagogogo/cnvd-skills`

**Risks:**
- GitHub 仓库重命名后旧 URL 仅短期 redirect，必须同步更新 `origin`，否则后续 push 失败 → 缓解：Task 1 立即 `git remote set-url` 指向新仓库地址
- 本地文件夹从 `cnvd-crawler` 改名为 `cnvd-skills` 会改变当前工作目录，命令中途 CWD 失效 → 缓解：Task 4 使用 `cd ..` 切到父目录后执行 `mv`，全命令用绝对/相对父目录路径，不依赖会失效的旧 CWD
- Go 包目录改名后，所有 `package cnvd_crawler` 声明与 import 路径必须一并改，否则 `go build` 报 `package not found` → 缓解：Task 2 用 `git mv` 改目录名 + `sed` 批量替换 package 声明与 import，Task 3 编译验证拦截
- `gh repo rename` 需要 `repo` scope → 已确认 token 含 `repo` scope（见调研）
- 改名前仓库已有本地改动可能导致远端不匹配 → 缓解：调研确认工作区干净（git status clean），直接执行

---

### Task 1: 重命名 GitHub 远端仓库并更新 origin remote

**Depends on:** None
**Files:**
- Modify: `.git/config`（origin remote URL）

- [ ] **Step 1: 调用 gh repo rename 重命名远端仓库 — 把 GitHub 仓库名从 cnvd-crawler 改为 cnvd-skills**

```bash
gh repo rename cnvd-skills --repo scagogogo/cnvd-crawler --yes
```

- [ ] **Step 2: 更新 origin remote URL 指向新仓库名 — 让后续 push/pull 指向 cnvd-skills**

```bash
git remote set-url origin git@github.com:scagogogo/cnvd-skills.git
git remote -v
```

- [ ] **Step 3: 验证远端改名与 remote 更新成功**
Run: `gh repo view scagogogo/cnvd-skills --json nameWithOwner,url`
Expected:
  - Exit code: 0
  - Output contains: `"nameWithOwner":"scagogogo/cnvd-skills"`
  - Output does NOT contain: `Could not resolve`

- [ ] **Step 4: 提交**
Run: `echo "remote-only change, no file commit needed"`

---

### Task 2: 重命名 Go 包目录并替换所有 package 声明与 import 路径

**Depends on:** Task 1
**Files:**
- Modify: `go.mod:1`
- Rename: `cnvd_crawler/` → `cnvd_skills/`（8 个文件）
- Modify: `cnvd_skills/*.go` 每个文件第 1 行 `package cnvd_crawler` → `package cnvd_skills`
- Modify: `main.go:5`

- [ ] **Step 1: 修改 go.mod 的 module 名 — 使 module 路径与新仓库名一致**
文件: `go.mod:1`

```text
module github.com/scagogogo/cnvd-skills
```

- [ ] **Step 2: 用 git mv 重命名包目录 — 把 cnvd_crawler/ 改为 cnvd_skills/，保留 git 历史**

```bash
git mv cnvd_crawler cnvd_skills
```

- [ ] **Step 3: 批量替换所有 .go 文件的 package 声明 — 将 package cnvd_crawler 改为 package cnvd_skills**

```bash
sed -i 's/^package cnvd_crawler$/package cnvd_skills/' cnvd_skills/*.go
```

- [ ] **Step 4: 修改 main.go 的 import 路径 — 指向新 module 名 + 新包目录名**
文件: `main.go:5`

```go
	"github.com/scagogogo/cnvd-skills/cnvd_skills"
```

- [ ] **Step 5: 验证目录改名与 package 替换**
Run: `grep -rn "^package" cnvd_skills/ && ls cnvd_skills/`
Expected:
  - Exit code: 0
  - Output contains: `package cnvd_skills`（8 行）
  - Output does NOT contain: `cnvd_crawler`

- [ ] **Step 6: 提交**
Run: `git add go.mod main.go cnvd_skills/ && git rm -r --cached cnvd_crawler 2>/dev/null; git commit -m "refactor(rename): rename package dir cnvd_crawler -> cnvd_skills and update module/import"`

---

### Task 3: 重命名导出符号 CnvdCrawler → CnvdSkills 并编译验证

**Depends on:** Task 2
**Files:**
- Modify: `cnvd_skills/cnvd_crawler.go`（重命名为 `cnvd_skills.go`，文件内 `CnvdCrawler`→`CnvdSkills`）
- Modify: `cnvd_skills/vul_list.go`（方法接收器 `*CnvdCrawler`→`*CnvdSkills`）
- Modify: `cnvd_skills/vul_detail.go`（方法接收器 `*CnvdCrawler`→`*CnvdSkills`）
- Modify: `cnvd_skills/vul_list_test.go`（`TestCnvdCrawler_VulList`→`TestCnvdSkills_VulList`，`NewCnvdCrawler`→`NewCnvdSkills`）
- Modify: `cnvd_skills/vul_detail_test.go`（`TestCnvdCrawler_RequestVulDetail`→`TestCnvdSkills_RequestVulDetail`，`NewCnvdCrawler`→`NewCnvdSkills`）
- Modify: `main.go:9`（`cnvd_crawler.NewCnvdCrawler`→`cnvd_skills.NewCnvdSkills`）

- [ ] **Step 1: 重命名核心文件 cnvd_crawler.go 为 cnvd_skills.go — 文件名跟随类型名**

```bash
git mv cnvd_skills/cnvd_crawler.go cnvd_skills/cnvd_skills.go
git mv cnvd_skills/cnvd_crawler_test.go cnvd_skills/cnvd_skills_test.go
```

- [ ] **Step 2: 替换全部源文件中的 CnvdCrawler/NewCnvdCrawler 标识符 — 改为 CnvdSkills/NewCnvdSkills**

```bash
sed -i 's/CnvdCrawler/CnvdSkills/g; s/NewCnvdCrawler/NewCnvdSkills/g' cnvd_skills/*.go main.go
```

- [ ] **Step 3: 验证符号替换无遗漏**
Run: `grep -rn "CnvdCrawler\|NewCnvdCrawler\|cnvd_crawler" . --include="*.go" --include="*.mod" | grep -v '/.git/'`
Expected:
  - Exit code: 1（grep 无匹配返回 1）
  - Output: 空（无任何匹配）

- [ ] **Step 4: go build 编译验证 — 确认 module/包/符号改名后可编译**
Run: `go build ./...`
Expected:
  - Exit code: 0
  - Output: 空（无报错）
  - Output does NOT contain: `cannot find package`、`undefined:`

- [ ] **Step 5: go vet 静态检查 — 确认改名未引入语法/语义问题**
Run: `go vet ./...`
Expected:
  - Exit code: 0
  - Output does NOT contain: `error`

- [ ] **Step 6: 提交**
Run: `git add cnvd_skills/ main.go && git commit -m "refactor(rename): rename exported symbols CnvdCrawler -> CnvdSkills"`

---

### Task 4: 重命名本地工作目录文件夹并验证 git 仍可用

**Depends on:** Task 3
**Files:**
- Rename: `/home/cc11001100/github/scagogogo/cnvd-crawler` → `/home/cc11001100/github/scagogogo/cnvd-skills`

- [ ] **Step 1: 切到父目录并重命名本地文件夹 — 把 cnvd-crawler 目录改名为 cnvd-skills**

```bash
cd /home/cc11001100/github/scagogogo && mv cnvd-crawler cnvd-skills && cd cnvd-skills && pwd
```

- [ ] **Step 2: 验证 git 仓库在新目录仍正常**
Run: `git -C /home/cc11001100/github/scagogogo/cnvd-skills status && git -C /home/cc11001100/github/scagogogo/cnvd-skills remote -v`
Expected:
  - Exit code: 0
  - Output contains: `origin	git@github.com:scagogogo/cnvd-skills.git`
  - Output contains: `位于分支 main` 或 `On branch main`

- [ ] **Step 3: 验证 go build 在新目录仍通过**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && go build ./...`
Expected:
  - Exit code: 0
  - Output: 空

- [ ] **Step 4: 提交**
Run: `echo "directory rename, no git commit needed"`

---

### Task 5: 推送到远端并验证远端仓库一致

**Depends on:** Task 4
**Files:**
- Push: 本地 `main` → `origin/main`

- [ ] **Step 1: 推送所有改名提交到远端 — 同步 module/包/符号改名到 GitHub**

```bash
cd /home/cc11001100/github/scagogogo/cnvd-skills && git push origin main
```

- [ ] **Step 2: 验证远端代码与本地一致**
Run: `cd /home/cc11001100/github/scagogogo/cnvd-skills && git log --oneline -5 && gh repo view scagogogo/cnvd-skills --json nameWithOwner,url`
Expected:
  - Exit code: 0
  - Output contains: `refactor(rename)`（最近两次提交信息）
  - Output contains: `"nameWithOwner":"scagogogo/cnvd-skills"`

- [ ] **Step 3: 提交**
Run: `echo "push only, no local commit needed"`
