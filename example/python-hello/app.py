import json
import os
import platform
import sys


def config_path():
    # 打包后配置解压在 sys._MEIPASS；开发时读 dist/config.json
    base = getattr(sys, "_MEIPASS", None)
    if base:
        return os.path.join(base, "config.json")
    return os.path.join(os.getcwd(), "dist", "config.json")


def load_config():
    with open(config_path(), encoding="utf-8") as f:
        return json.load(f)


def greet(name, cfg):
    return (
        f"python-hello: {name} "
        f"({platform.system().lower()}/{platform.machine()}, "
        f"env={cfg['env']}, port={cfg['port']})"
    )


def main():
    name = sys.argv[1] if len(sys.argv) > 1 else "world"
    print(greet(name, load_config()))


if __name__ == "__main__":
    main()
