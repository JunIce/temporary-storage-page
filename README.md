# MinIO 文件上传服务器

这是一个基于 Express.js 和 MinIO 的文件上传服务器，支持大文件分片上传、断点续传等功能。

## 功能特性

- 🚀 大文件分片上传
- ⏸️ 断点续传
- 📁 拖拽上传
- 📊 实时上传进度
- 📝 上传历史记录
- 🔄 自动重试机制
- 📱 响应式设计

## 快速开始

### 1. 安装依赖

```bash
npm install
```

### 2. 启动服务器

```bash
# 开发模式
npm run dev

# 生产模式
npm start
```

### 3. 访问应用

打开浏览器访问 `http://localhost:3000`

## API 接口

### 初始化分片上传

```http
POST /api/upload/init
Content-Type: application/json

{
  "fileName": "example.pdf",
  "fileSize": 10485760,
  "chunkSize": 5242880
}
```

### 上传分片

```http
POST /api/upload/chunk
Content-Type: multipart/form-data

uploadId: uuid
chunkIndex: 0
chunk: [文件数据]
```

### 完成上传

```http
POST /api/upload/complete
Content-Type: application/json

{
  "uploadId": "uuid"
}
```

### 取消上传

```http
DELETE /api/upload/:uploadId
```

### 获取上传状态

```http
GET /api/upload/:uploadId/status
```

### 单文件上传（小文件）

```http
POST /api/upload/single
Content-Type: multipart/form-data

file: [文件数据]
```

### 获取文件列表

```http
GET /api/files
```

### 删除文件

```http
DELETE /api/files/:objectName
```

## 配置说明

### MinIO 配置

在 `server.js` 中修改 MinIO 连接配置：

```javascript
const minioConfig = {
    endPoint: 'your-minio-server',
    port: 9000,
    useSSL: false,
    accessKey: 'your-access-key',
    secretKey: 'your-secret-key'
};
```

### 前端配置

在 `index.html` 中修改 MinIO 配置：

```javascript
const MINIO_CONFIG = {
    endPoint: 'http://your-minio-server:9001/',
    port: 9000,
    useSSL: false,
    accessKey: 'your-access-key',
    secretKey: 'your-secret-key',
    bucket: 'temporary-files'
};
```

## 项目结构

```
minio-server/
├── package.json          # 项目依赖配置
├── server.js             # Express 服务器
├── index.html            # 前端上传界面
└── README.md             # 项目说明
```

## 技术栈

### 后端
- **Express.js** - Web 框架
- **MinIO** - 对象存储服务
- **Multer** - 文件上传中间件
- **UUID** - 生成唯一标识符

### 前端
- **原生 JavaScript** - 核心逻辑
- **HTML5** - 页面结构
- **CSS3** - 样式设计
- **IndexedDB** - 本地数据存储

## 注意事项

1. 确保 MinIO 服务器正在运行并且可以访问
2. 检查 MinIO 的访问密钥和存储桶权限
3. 大文件上传会占用较多内存，建议适当调整分片大小
4. 生产环境中建议使用 HTTPS 和更安全的认证机制

## 故障排除

### 常见问题

1. **连接 MinIO 失败**
   - 检查 MinIO 服务器地址和端口
   - 验证访问密钥是否正确
   - 确认网络连接正常

2. **文件上传失败**
   - 检查文件大小是否超过限制
   - 确认存储桶是否存在且有写权限
   - 查看服务器日志获取详细错误信息

3. **分片上传问题**
   - 检查分片大小设置
   - 确认所有分片都已成功上传
   - 验证分片合并逻辑

## 许可证

MIT License