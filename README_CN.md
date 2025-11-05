# MinIO 文件暂存服务

一个功能强大的基于 Express.js 和 MinIO 的文件上传服务器，专为现代Web应用设计，支持大文件分片上传、断点续传、多语言界面等企业级功能。

## 截图

![页面截图](./screenshots/page.png)

## ✨ 核心特性

### 🚀 文件上传功能
- **大文件分片上传** - 支持GB级大文件稳定上传
- **断点续传** - 网络中断后可从断点继续上传
- **拖拽上传** - 直观的拖拽文件上传体验
- **批量上传** - 支持多文件同时上传
- **实时进度** - 显示上传速度、进度百分比、剩余时间
- **自动重试** - 上传失败自动重试机制

### 🎨 用户界面
- **响应式设计** - 完美适配桌面、平板、手机
- **现代化UI** - 采用渐变色彩和流畅动画
- **多语言支持** - 支持中文、英文界面切换
- **暗色主题** - 护眼的暗色界面选项
- **触摸优化** - 移动端友好的触摸交互

### 📊 数据管理
- **上传历史** - 本地存储上传记录
- **文件管理** - 查看、下载、删除已上传文件
- **状态追踪** - 实时显示上传状态（上传中/已完成/失败）
- **存储提示** - 文件保存期限提醒

### 🔒 安全特性
- **文件验证** - 文件类型和大小验证
- **临时存储** - 文件定时自动清理
- **访问控制** - 基于密钥的MinIO访问控制
- **HTTPS支持** - 生产环境安全传输

## 🚀 快速开始

### 环境要求

- Node.js 14.0 或更高版本
- MinIO 服务器
- 现代浏览器（Chrome 80+、Firefox 75+、Safari 13+）

### 1. 克隆项目

```bash
git clone https://github.com/your-username/minio-server.git
cd minio-server
```

### 2. 安装依赖

```bash
npm install
```

### 3. 配置环境变量

创建 `.env` 文件：

```env
# MinIO 配置
MINIO_ENDPOINT=localhost
MINIO_PORT=9000
MINIO_ACCESS_KEY=your-access-key
MINIO_SECRET_KEY=your-secret-key
MINIO_BUCKET=temporary-files
MINIO_USE_SSL=false

# 服务器配置
PORT=3000
NODE_ENV=development

# 文件配置
MAX_FILE_SIZE=1073741824  # 1GB
CHUNK_SIZE=5242880       # 5MB
FILE_EXPIRY_DAYS=2       # 2天
```

### 4. 启动服务器

```bash
# 开发模式（热重载）
npm run dev

# 生产模式
npm start

# 调试模式
npm run debug
```

### 5. 访问应用

打开浏览器访问 `http://localhost:3000`

## 📚 API 文档

### 分片上传流程

#### 1. 初始化分片上传

```http
POST /api/upload/init
Content-Type: application/json

{
  "fileName": "large-file.mp4",
  "fileSize": 1073741824,
  "chunkSize": 5242880,
  "totalChunks": 205,
  "fileType": "video/mp4"
}
```

**响应：**
```json
{
  "success": true,
  "uploadId": "uuid-upload-id",
  "chunkSize": 5242880,
  "totalChunks": 205
}
```

#### 2. 上传分片

```http
POST /api/upload/chunk
Content-Type: multipart/form-data

uploadId: uuid-upload-id
chunkIndex: 0
chunk: [二进制文件数据]
```

**响应：**
```json
{
  "success": true,
  "chunkIndex": 0,
  "uploaded": true,
  "progress": 0.49
}
```

#### 3. 完成上传

```http
POST /api/upload/complete
Content-Type: application/json

{
  "uploadId": "uuid-upload-id",
  "fileName": "large-file.mp4",
  "totalChunks": 205
}
```

**响应：**
```json
{
  "success": true,
  "objectName": "uuid-large-file.mp4",
  "url": "/files/uuid-large-file.mp4",
  "message": "文件上传完成"
}
```

### 其他API接口

#### 取消上传
```http
DELETE /api/upload/:uploadId
```

#### 获取上传状态
```http
GET /api/upload/:uploadId/status
```

#### 单文件上传（小文件）
```http
POST /api/upload/single
Content-Type: multipart/form-data

file: [文件数据]
```

#### 获取文件列表
```http
GET /api/files?page=1&limit=20
```

#### 下载文件
```http
GET /api/files/:objectName/download
```

#### 删除文件
```http
DELETE /api/files/:objectName
```


## 🏗️ 项目结构

```
minio-server/
├── package.json              # 项目依赖和脚本
├── package-lock.json         # 锁定依赖版本
├── server.js                 # Express 服务器主文件
├── index.html                # 前端上传界面
├── .env                      # 环境变量配置
├── .env.example              # 环境变量示例
├── .gitignore               # Git 忽略文件
├── README_CN.md             # 中文文档
├── README.md                # 英文文档
├── node_modules/            # Node.js 依赖
└── logs/                    # 日志文件目录
    ├── access.log           # 访问日志
    ├── error.log            # 错误日志
    └── upload.log           # 上传日志
```

## 🛠️ 技术栈

### 后端技术
- **Express.js** - 快速、极简的 Web 框架
- **MinIO** - 高性能对象存储服务
- **Multer** - 文件上传中间件
- **UUID** - 生成唯一标识符
- **Helmet** - 安全中间件
- **CORS** - 跨域资源共享

### 前端技术
- **原生 JavaScript (ES6+)** - 现代JavaScript特性
- **HTML5** - 语义化标记
- **CSS3** - 现代样式和动画
- **IndexedDB** - 浏览器本地数据库
- **Fetch API** - 现代HTTP请求
- **Service Worker** - 离线支持

## 🔧 开发指南

### 本地开发

1. **安装开发依赖**
   ```bash
   npm install --dev
   ```

2. **启动开发服务器**
   ```bash
   npm run start
   ```

### 生产部署

1. **启动生产服务器**
   ```bash
   npm start
   ```

2. **使用 PM2 管理进程**
   ```bash
   pm2 start server.js --name minio-server
   ```

### Docker 部署

```dockerfile
FROM node:16-alpine

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production

COPY . .

EXPOSE 3000

CMD ["npm", "start"]
```

```bash
# 构建镜像
docker build -t minio-server .

# 运行容器
docker run -p 3000:3000 --env-file .env minio-server
```

## 📝 使用示例

### JavaScript 客户端示例

```javascript
// 初始化上传
const initUpload = async (file) => {
  const response = await fetch('/api/upload/init', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      fileName: file.name,
      fileSize: file.size,
      chunkSize: 5242880
    })
  });
  
  return await response.json();
};

// 上传分片
const uploadChunk = async (uploadId, chunkIndex, chunk) => {
  const formData = new FormData();
  formData.append('uploadId', uploadId);
  formData.append('chunkIndex', chunkIndex);
  formData.append('chunk', chunk);
  
  const response = await fetch('/api/upload/chunk', {
    method: 'POST',
    body: formData
  });
  
  return await response.json();
};
```

### cURL 示例

```bash
# 初始化上传
curl -X POST http://localhost:3000/api/upload/init \
  -H "Content-Type: application/json" \
  -d '{"fileName":"test.pdf","fileSize":1048576,"chunkSize":524288}'

# 上传分片
curl -X POST http://localhost:3000/api/upload/chunk \
  -F "uploadId=uuid-here" \
  -F "chunkIndex=0" \
  -F "chunk=@chunk_0.bin"

# 完成上传
curl -X POST http://localhost:3000/api/upload/complete \
  -H "Content-Type: application/json" \
  -d '{"uploadId":"uuid-here","fileName":"test.pdf","totalChunks":2}'
```

### 代码规范

- 使用 ESLint 和 Prettier
- 遵循 JavaScript Standard Style
- 编写单元测试
- 更新相关文档

### 问题报告

使用 GitHub Issues 报告问题，请包含：
- 详细的问题描述
- 复现步骤
- 环境信息
- 错误日志

## 📄 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 🙏 致谢

感谢以下开源项目：
- [Express.js](https://expressjs.com/) - Web 框架
- [MinIO](https://min.io/) - 对象存储
- [Multer](https://github.com/expressjs/multer) - 文件上传
