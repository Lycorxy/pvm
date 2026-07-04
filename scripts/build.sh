#!/usr/bin/env bash
# pvm 跨平台打包脚本 (Linux/macOS Bash)
# 用法: ./scripts/build.sh [-v x.x.x] [-c]

set -euo pipefail

VERSION=""
CURRENT_ONLY=false

while getopts "v:c" opt; do
    case $opt in
        v) VERSION="$OPTARG" ;;
        c) CURRENT_ONLY=true ;;
        *) echo "用法: $0 [-v 版本号] [-c(仅当前平台)]"; exit 1 ;;
    esac
done

# 自动从 cmd/root.go 获取版本号
if [ -z "$VERSION" ]; then
    VERSION=$(grep -oP 'Version\s*=\s*"\K[^"]+' cmd/root.go 2>/dev/null || echo "dev")
fi

LDFLAGS="-s -w -X github.com/pvm/pvm/cmd.Version=$VERSION"
OUTPUT_DIR="dist"

# 清理旧的构建产物
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

echo ""
echo "======================================"
echo "  pvm v$VERSION 跨平台打包"
echo "======================================"
echo ""

if [ "$CURRENT_ONLY" = true ]; then
    # 只编译当前平台
    echo "[1/1] 编译当前平台..."
    go build -ldflags "$LDFLAGS" -o "$OUTPUT_DIR/pvm" .
    echo "  -> $OUTPUT_DIR/pvm"
else
    # 跨平台编译
    PLATFORMS=(
        "windows/amd64/.exe"
        "windows/arm64/.exe"
        "darwin/amd64"
        "darwin/arm64"
        "linux/amd64"
        "linux/arm64"
    )

    TOTAL=${#PLATFORMS[@]}
    SUCCESS=0
    FAILED=0
    HASHES=""

    for i in "${!PLATFORMS[@]}"; do
        IFS='/' read -r GOOS GOARCH EXT <<< "${PLATFORMS[$i]}"
        [ -z "$EXT" ] && EXT=""
        NAME="${GOOS}-${GOARCH}"
        OUTPUT="$OUTPUT_DIR/pvm-${NAME}${EXT}"
        NUM=$((i + 1))

        echo "[$NUM/$TOTAL] 编译 $NAME..."

        if GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "$LDFLAGS" -o "$OUTPUT" .; then
            SIZE=$(du -h "$OUTPUT" | cut -f1)
            echo "  -> $OUTPUT ($SIZE)"
            SUCCESS=$((SUCCESS + 1))

            # 计算校验和
            if command -v sha256sum &>/dev/null; then
                HASH=$(sha256sum "$OUTPUT" | cut -d' ' -f1)
            else
                HASH=$(shasum -a 256 "$OUTPUT" | cut -d' ' -f1)
            fi
            HASHES="${HASHES}${HASH}  pvm-${NAME}${EXT}"$'\n'
        else
            echo "  -> 失败！"
            FAILED=$((FAILED + 1))
        fi
    done

    # 生成 SHA256 校验和
    echo ""
    echo "生成 SHA256 校验和..."
    echo -n "$HASHES" > "$OUTPUT_DIR/checksums.txt"
    echo "  -> $OUTPUT_DIR/checksums.txt"

    echo ""
    echo "======================================"
    if [ $FAILED -gt 0 ]; then
        echo "  打包完成: $SUCCESS 成功, $FAILED 失败"
    else
        echo "  打包完成: $SUCCESS 成功, $FAILED 失败"
    fi
    echo "======================================"
fi

echo ""
