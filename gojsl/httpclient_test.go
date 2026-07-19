package jsl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHttpClient_SetCookie_ReadBack(t *testing.T) {
	hc := NewHttpClient("", 0)
	hc.SetCookie("https://www.cnvd.org.cn/flaw/list", "__jsl_clearance_s", "v123")
	cs := hc.Cookies("https://www.cnvd.org.cn/flaw/list")
	found := false
	for _, c := range cs {
		if c.Name == "__jsl_clearance_s" && c.Value == "v123" {
			found = true
		}
	}
	assert.True(t, found, "SetCookie 写入后应能从 jar 读回")
}

func TestHttpClient_Do_GetReturnsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.Header.Get("User-Agent"))
		assert.NotEmpty(t, r.Header.Get("Sec-Fetch-Site"))
		w.Write([]byte("hello"))
	}))
	defer srv.Close()
	hc := NewHttpClient("", 0)
	body, err := hc.Do(context.Background(), srv.URL, nil)
	assert.Nil(t, err)
	assert.Equal(t, "hello", body)
}

func TestHttpClient_DoPost_BodyAndContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		_ = r.ParseForm()
		assert.Equal(t, "42", r.FormValue("ans"))
		w.Write([]byte("submitted"))
	}))
	defer srv.Close()
	hc := NewHttpClient("", 0)
	body, err := hc.DoPost(context.Background(), srv.URL, "ans=42", nil)
	assert.Nil(t, err)
	assert.Equal(t, "submitted", body)
}

func TestHttpClient_DoStatus_ReturnsStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	hc := NewHttpClient("", 0)
	body, status, err := hc.DoStatus(context.Background(), srv.URL, nil)
	assert.Nil(t, err)
	assert.Equal(t, 200, status)
	assert.Equal(t, "ok", body)
}

func TestHttpClient_DoPostStatus_Non200ReturnsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"msg":"args-err"}`))
	}))
	defer srv.Close()
	hc := NewHttpClient("", 0)
	body, status, err := hc.DoPostStatus(context.Background(), srv.URL, "ans=wrong", nil)
	assert.Nil(t, err)
	assert.Equal(t, 401, status)
	assert.Contains(t, body, "args-err")
}

func TestHttpClient_RefreshUserAgent_KeepsHeaders(t *testing.T) {
	hc := NewHttpClient("", 0)
	hc.RefreshUserAgent()
	// 刷新后通过一次请求验证 UA 头仍存在
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.Header.Get("User-Agent"))
		assert.Contains(t, r.Header.Get("sec-ch-ua"), "Chromium")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	_, err := hc.Do(context.Background(), srv.URL, nil)
	assert.Nil(t, err)
}
