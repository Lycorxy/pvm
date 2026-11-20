'use strict';

// 按环境生成配置文件：node scripts/gen-config.js dev|test|pre|pro
const fs = require('fs');
const path = require('path');

const env = process.argv[2] || 'dev';
const ports = { dev: 3000, test: 4000, pre: 5000, pro: 8080 };
const config = { env, port: ports[env] ?? 3000 };

fs.mkdirSync('dist', { recursive: true });
fs.writeFileSync(path.join('dist', 'config.json'), JSON.stringify(config, null, 2) + '\n');
console.log(`wrote dist/config.json (env=${config.env}, port=${config.port})`);
