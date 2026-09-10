# --- Stage 1: Build Vue App ---
# 使用一个包含Node.js和npm的镜像
FROM node:20-alpine AS builder

WORKDIR /app

# 复制package.json和package-lock.json并安装所有依赖
COPY package*.json ./
RUN npm install

# 复制所有源代码
COPY . .

# 执行构建，生成dist目录
RUN npm run build

# --- Stage 2: Final Image ---
# 使用一个轻量级的Node.js镜像来运行服务器
FROM node:20-alpine

WORKDIR /app

# 只复制生产环境需要的依赖定义和服务器代码
COPY package*.json ./
COPY server.js ./

# 只安装生产依赖，不安装开发依赖（如@vue/cli）
RUN npm install --omit=dev

# 从构建阶段复制打包好的dist目录
COPY --from=builder /app/dist ./dist

# 暴露前端服务器监听的3000端口
EXPOSE 3000

# 设置容器启动命令
CMD ["node", "server.js"]
