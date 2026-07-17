package jsl

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CaptchaSolver 把验证码图片（PNG 字节）翻译成答案字符串。
// 库内自动完成取图、提交、放行刷新，仅把"图→答案"这一步留给实现。
// 返回 error 表示无法识别，库会换一张图重试。
type CaptchaSolver interface {
	// Solve 接收 base64 编码的 PNG 图片数据（与 CNVD captcha 端点返回的 image 字段同格式），
	// 返回识别出的答案字符串。ctx 用于取消。
	Solve(ctx context.Context, imageBase64 string) (string, error)
}

// 验证码相关错误。调用方可用 errors.Is 判断。
var (
	// ErrCaptchaRequired 请求被验证码挑战拦截，但未配置 CaptchaSolver。
	ErrCaptchaRequired = errors.New("captcha challenge required but no solver configured")
	// ErrCaptchaSolveFailed 配置了 solver 但多次识别/提交均失败。
	ErrCaptchaSolveFailed = errors.New("captcha solve failed after retries")
)

// NoopCaptchaSolver 永不识别，Solve 返回 error。
// 用于明确要求调用方配置识别器的场景：遇到验证码即上抛 ErrCaptchaRequired。
type NoopCaptchaSolver struct{}

func (NoopCaptchaSolver) Solve(ctx context.Context, imageBase64 string) (string, error) {
	return "", ErrCaptchaRequired
}

// InteractiveCaptchaSolver 半自动识别器：把验证码图写到磁盘临时文件，
// 然后轮询环境变量 answerEnv（默认 CNVD_CAPTCHA_ANSWER）等待人工或外部脚本填入答案。
// 读到答案后清空环境变量并返回。适合交互/调试场景。
type InteractiveCaptchaSolver struct {
	// 答案环境变量名，默认 CNVD_CAPTCHA_ANSWER
	AnswerEnv string
	// 验证码图保存目录，默认 os.TempDir()
	ImageDir string
	// 等待答案的最长时间，默认 5 分钟
	WaitTimeout time.Duration
	// 轮询间隔，默认 1 秒
	PollInterval time.Duration
}

// Solve 实现 CaptchaSolver：写图、轮询环境变量、返回答案。
func (s InteractiveCaptchaSolver) Solve(ctx context.Context, imageBase64 string) (string, error) {
	envName := s.AnswerEnv
	if envName == "" {
		envName = "CNVD_CAPTCHA_ANSWER"
	}
	pollInterval := s.PollInterval
	if pollInterval == 0 {
		pollInterval = 1 * time.Second
	}
	waitTimeout := s.WaitTimeout
	if waitTimeout == 0 {
		waitTimeout = 5 * time.Minute
	}

	// 1. 把图片写到临时文件供人工查看
	imgDir := s.ImageDir
	if imgDir == "" {
		imgDir = os.TempDir()
	}
	_ = os.MkdirAll(imgDir, 0o755)
	if pngBytes, err := base64.StdEncoding.DecodeString(imageBase64); err == nil {
		imgPath := filepath.Join(imgDir, fmt.Sprintf("cnvd_captcha_%d.png", time.Now().UnixNano()))
		if err := os.WriteFile(imgPath, pngBytes, 0o644); err == nil {
			fmt.Printf("[captcha] 验证码图已保存到 %s，请识别后设置环境变量 %s\n", imgPath, envName)
		}
	}

	// 2. 轮询环境变量等待答案（ctx 感知）
	deadline := time.Now().Add(waitTimeout)
	for {
		if ans := os.Getenv(envName); ans != "" {
			_ = os.Setenv(envName, "")
			return ans, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("%w: timeout waiting for %s", ErrCaptchaSolveFailed, envName)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// StaticCaptchaSolver 返回固定答案，仅供单测使用。
type StaticCaptchaSolver struct {
	Answer string
	Err    error
}

func (s StaticCaptchaSolver) Solve(ctx context.Context, imageBase64 string) (string, error) {
	return s.Answer, s.Err
}

// CommandCaptchaSolver 通过调用外部命令识别验证码：把 base64 图片通过 stdin 传入命令，
// 从 stdout 读取答案。适合把 Python 的 ddddocr 等专用 OCR 包装成命令供 Go 调用，
// 保持本库为纯 Go，识别能力按需接入。命令退出码非 0 视为识别失败。
//
// 用法示例（搭配 scripts/ddddocr_solver.py）：
//
//	solver := CommandCaptchaSolver{Command: "python3", Args: []string{"scripts/ddddocr_solver.py"}}
type CommandCaptchaSolver struct {
	// Command 可执行命令名，如 "python3"
	Command string
	// Args 命令参数
	Args []string
}

// Solve 实现 CaptchaSolver：起子进程，stdin 写 base64，stdout 读答案。
func (s CommandCaptchaSolver) Solve(ctx context.Context, imageBase64 string) (string, error) {
	cmd := exec.CommandContext(ctx, s.Command, s.Args...)
	cmd.Stdin = strings.NewReader(imageBase64)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("captcha command failed: %w; stderr: %s", err, strings.TrimSpace(errBuf.String()))
	}
	ans := strings.TrimSpace(out.String())
	if ans == "" {
		return "", fmt.Errorf("captcha command returned empty answer; stderr: %s", strings.TrimSpace(errBuf.String()))
	}
	return ans, nil
}
