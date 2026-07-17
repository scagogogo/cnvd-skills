#!/usr/bin/env python3
"""ddddocr 验证码识别命令行包装。
从 stdin 读取 base64 编码的 PNG 图片，识别后把答案写到 stdout。
供 Go 库的 CommandCaptchaSolver 调用。

用法: echo <base64> | python3 ddddocr_solver.py
"""
import sys
import base64

def main():
    try:
        import ddddocr
    except Exception as e:
        print(f"ERR_IMPORT:{e}", file=sys.stderr)
        sys.exit(2)
    raw = sys.stdin.read().strip()
    if not raw:
        print("ERR_EMPTY_INPUT", file=sys.stderr)
        sys.exit(2)
    try:
        png = base64.b64decode(raw)
        ocr = ddddocr.DdddOcr(show_ad=False)
        ans = ocr.classification(png)
        # 只保留字母数字，去噪
        clean = "".join(c for c in ans if c.isalnum())
        sys.stdout.write(clean)
        sys.stdout.flush()
    except Exception as e:
        print(f"ERR_CLASSIFY:{e}", file=sys.stderr)
        sys.exit(3)

if __name__ == "__main__":
    main()
