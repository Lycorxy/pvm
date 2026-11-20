'use strict';

const fs = require('fs');
const path = require('path');

// 打包后：配置在 SEA blob 里（sea.getAsset）；开发时：读 dist/config.json
function loadConfig() {
  try {
    const { getAsset } = require('node:sea');
    return JSON.parse(getAsset('config.json', 'utf8'));
  } catch {
    return JSON.parse(fs.readFileSync(path.join(process.cwd(), 'dist', 'config.json'), 'utf8'));
  }
}

function greet(name, cfg) {
  return `node-hello: ${name} (${process.platform}/${process.arch}, env=${cfg.env}, port=${cfg.port})`;
}

const cfg = loadConfig();
console.log(greet(process.argv[2] || 'world', cfg));
