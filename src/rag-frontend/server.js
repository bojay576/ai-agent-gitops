// server.js
const express = require('express');
const { createProxyMiddleware } = require('http-proxy-middleware');
const path = require('path');

const app = express();
const port = 3000; // 前端服务器运行在3000端口
//const goApiTarget = 'http://localhost:8080'; // 我们的Go后端API地址
const goApiTarget = 'http://rag-backend-svc.default.svc.cluster.local:8080';

// 设置API代理
// 所有对/api/*的请求，都会被转发到Go后端
app.use('/api', createProxyMiddleware({
  target: goApiTarget,
  changeOrigin: true,
  pathRewrite: {
   // '^/api': '/api', // 保持路径不变
    '^/api': '', // 核心改动：将/api前缀替换为空字符串	  
  },
}));

// 提供Vue构建的静态文件
// __dirname是当前文件所在目录，'dist'是vue build的输出目录
app.use(express.static(path.join(__dirname, 'dist')));

// 对于任何未匹配到静态文件或API的请求，都返回index.html
// 这是单页面应用（SPA）的标准做
app.use((req, res) => {
  res.sendFile(path.join(__dirname, 'dist', 'index.html'));
});

app.listen(port, () => {
  console.log(`Frontend server with API proxy is listening at http://localhost:${port}`);
});
