# New API

新一代大模型网关与 AI 资产管理系统，聚合 40+ 上游 AI 提供商（OpenAI、Claude、Gemini、Azure、AWS Bedrock 等），提供用户管理、计费、速率限制和管理后台。

## 技术栈

- **后端**: Go 1.22+、Gin Web 框架、GORM v2 ORM
- **前端**: React 18、Vite、Semi Design UI (@douyinfe/semi-ui)
- **数据库**: SQLite、MySQL、PostgreSQL
- **缓存**: Redis (go-redis) + 内存缓存
- **认证**: JWT、WebAuthn/Passkeys、OAuth（GitHub、Discord、OIDC 等）
- **前端包管理器**: Bun

## 快速开始

### Docker 部署（推荐）

```bash
# 克隆项目
git clone https://github.com/QuantumNous/new-api.git
cd new-api

# 配置环境变量
cp .env.example .env
nano .env

# 本地编译（需要安装 Go 1.25+ 和 Bun）
task build

# 启动服务
docker-compose up -d
```

访问 `http://localhost:3000` 即可使用。

### 本地运行

```bash
# 克隆项目
git clone https://github.com/QuantumNous/new-api.git
cd new-api

# 配置环境变量
cp .env.example .env
nano .env

# 构建前端并启动后端
task all
```

访问 `http://localhost:3000` 即可使用。

### 编译说明

**本地编译命令**：

```bash
# 同时构建前端和后端
task build

# 仅构建前端
task build-frontend

# 仅启动后端开发服务器
task start-backend
```

编译后会在项目根目录生成 `terln-api` 二进制文件（包含前端静态文件）。

## 项目约定

详见 [AGENTS.md](./AGENTS.md)

## 许可证

本项目采用 [GNU Affero 通用公共许可证 v3.0 (AGPLv3)](./LICENSE) 授权。

**[官方文档](https://docs.newapi.pro/zh/docs)** • **[原项目](https://github.com/Calcium-Ion/new-api)**
