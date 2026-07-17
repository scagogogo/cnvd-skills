package jsl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNoopCaptchaSolver_ReturnsErrCaptchaRequired(t *testing.T) {
	_, err := NoopCaptchaSolver{}.Solve(context.Background(), "")
	assert.True(t, errors.Is(err, ErrCaptchaRequired))
}

func TestStaticCaptchaSolver_ReturnsAnswerOrErr(t *testing.T) {
	s := StaticCaptchaSolver{Answer: "abcd"}
	ans, err := s.Solve(context.Background(), "img")
	assert.Nil(t, err)
	assert.Equal(t, "abcd", ans)

	sErr := StaticCaptchaSolver{Err: ErrCaptchaSolveFailed}
	_, err = sErr.Solve(context.Background(), "img")
	assert.True(t, errors.Is(err, ErrCaptchaSolveFailed))
}

// TestInteractiveCaptchaSolver_ReadsEnvAnswer 预置环境变量后应立即返回
func TestInteractiveCaptchaSolver_ReadsEnvAnswer(t *testing.T) {
	envName := "CNVD_TEST_CAPTCHA_ANS"
	t.Setenv(envName, "1234")
	s := InteractiveCaptchaSolver{AnswerEnv: envName, PollInterval: 50 * time.Millisecond}
	ans, err := s.Solve(context.Background(), "")
	assert.Nil(t, err)
	assert.Equal(t, "1234", ans)
	// 读后应清空
	assert.Empty(t, os.Getenv(envName))
}

// TestInteractiveCaptchaSolver_ContextCancel 返回 ctx 错误
func TestInteractiveCaptchaSolver_ContextCancel(t *testing.T) {
	envName := "CNVD_TEST_CAPTCHA_NEVER"
	t.Setenv(envName, "")
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	s := InteractiveCaptchaSolver{AnswerEnv: envName, PollInterval: 30 * time.Millisecond, WaitTimeout: time.Minute}
	_, err := s.Solve(ctx, "")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestInteractiveCaptchaSolver_WritesImageFile 能解码 base64 并写文件
func TestInteractiveCaptchaSolver_WritesImageFile(t *testing.T) {
	// 1x1 透明 PNG 的 base64
	pngB64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mNk+M8AAAMAAX2BmM0AAAAASUVORK5CYII="
	dir := t.TempDir()
	s := InteractiveCaptchaSolver{
		AnswerEnv:    "CNVD_TEST_CAPTCHA_IMG_NEVER",
		ImageDir:     dir,
		PollInterval: 20 * time.Millisecond,
		WaitTimeout:  60 * time.Millisecond,
	}
	_, err := s.Solve(context.Background(), pngB64)
	assert.Error(t, err) // 超时
	// 文件应已写出
	matches, _ := filepath.Glob(filepath.Join(dir, "cnvd_captcha_*.png"))
	assert.NotEmpty(t, matches, "应写出验证码图文件")
}
