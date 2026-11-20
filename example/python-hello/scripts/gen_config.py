"""按环境生成配置文件：python scripts/gen_config.py dev|test|pre|pro"""

import json
import os
import sys

env = sys.argv[1] if len(sys.argv) > 1 else "dev"
ports = {"dev": 3000, "test": 4000, "pre": 5000, "pro": 8080}
config = {"env": env, "port": ports.get(env, 3000)}

os.makedirs("dist", exist_ok=True)
with open(os.path.join("dist", "config.json"), "w", encoding="utf-8") as f:
    json.dump(config, f, ensure_ascii=False, indent=2)
    f.write("\n")

print(f"wrote dist/config.json (env={config['env']}, port={config['port']})")
