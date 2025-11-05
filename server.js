require('dotenv').config();
const express = require('express');
const multer = require('multer');
const Minio = require('minio');
const cors = require('cors');
const { v4: uuidv4 } = require('uuid');
const crypto = require('crypto');
const path = require('path');

const app = express();
const PORT = process.env.PORT || 3000;

// 中间件配置
app.use(cors());
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// MinIO配置
const minioConfig = {
    endPoint: process.env.MINIO_ENDPOINT,
    port: parseInt(process.env.MINIO_PORT),
    useSSL: process.env.MINIO_USE_SSL === 'true',
    accessKey: process.env.MINIO_ACCESS_KEY,
    secretKey: process.env.MINIO_SECRET_KEY
};

// 初始化MinIO客户端
const minioClient = new Minio.Client(minioConfig);

const bucketName = process.env.BUCKET_NAME;

// 存储分片上传信息
const multipartUploads = new Map();

// 初始化存储桶
async function initializeBucket() {
    try {
        const exists = await minioClient.bucketExists(bucketName);
        if (!exists) {
            await minioClient.makeBucket(bucketName, 'us-east-1');
            console.log(`存储桶 ${bucketName} 创建成功`);
        } else {
            console.log(`存储桶 ${bucketName} 已存在`);
        }
    } catch (error) {
        console.error('初始化存储桶失败:', error);
    }
}

// 配置multer用于处理文件上传
const storage = multer.memoryStorage();
const upload = multer({ 
    storage: storage,
    limits: {
        fileSize: 100 * 1024 * 1024 // 100MB限制
    }
});

// 静态文件服务 - 提供index.html
app.get('/', (req, res) => {
    res.sendFile(path.join(__dirname, 'index.html'));
});

// 初始化分片上传
app.post('/api/upload/init', async (req, res) => {
    try {
        const { fileName, fileSize, chunkSize } = req.body;
        
        if (!fileName || !fileSize || !chunkSize) {
            return res.status(400).json({ 
                error: '缺少必要参数: fileName, fileSize, chunkSize' 
            });
        }

        const uploadId = uuidv4();
        const objectName = `${Date.now()}_${fileName}`;
        
        // 存储分片上传信息
        multipartUploads.set(uploadId, {
            fileName,
            objectName,
            fileSize: parseInt(fileSize),
            chunkSize: parseInt(chunkSize),
            totalChunks: Math.ceil(fileSize / chunkSize),
            uploadedChunks: new Set(),
            createdAt: new Date()
        });

        res.json({
            uploadId,
            objectName,
            totalChunks: Math.ceil(fileSize / chunkSize)
        });
    } catch (error) {
        console.error('初始化分片上传失败:', error);
        res.status(500).json({ error: '初始化上传失败' });
    }
});

// 上传分片
app.post('/api/upload/chunk', upload.single('chunk'), async (req, res) => {
    try {
        const { uploadId, chunkIndex } = req.body;
        const chunk = req.file;

        if (!uploadId || chunkIndex === undefined || !chunk) {
            return res.status(400).json({ 
                error: '缺少必要参数: uploadId, chunkIndex, chunk' 
            });
        }

        const uploadInfo = multipartUploads.get(uploadId);
        if (!uploadInfo) {
            return res.status(404).json({ error: '上传任务不存在' });
        }

        const chunkIndexNum = parseInt(chunkIndex);
        
        // 生成分片对象名
        const chunkObjectName = `${uploadInfo.objectName}.part${chunkIndexNum}`;
        
        // 上传分片到MinIO
        await minioClient.putObject(bucketName, chunkObjectName, chunk.buffer);
        
        // 记录已上传的分片
        uploadInfo.uploadedChunks.add(chunkIndexNum);
        
        console.log(`分片 ${chunkIndexNum} 上传成功，对象名: ${chunkObjectName}`);

        res.json({
            success: true,
            chunkIndex: chunkIndexNum,
            uploadedChunks: uploadInfo.uploadedChunks.size,
            totalChunks: uploadInfo.totalChunks
        });
    } catch (error) {
        console.error('上传分片失败:', error);
        res.status(500).json({ error: '分片上传失败' });
    }
});

// 完成分片上传
app.post('/api/upload/complete', async (req, res) => {
    try {
        const { uploadId } = req.body;

        if (!uploadId) {
            return res.status(400).json({ error: '缺少uploadId参数' });
        }

        const uploadInfo = multipartUploads.get(uploadId);
        if (!uploadInfo) {
            return res.status(404).json({ error: '上传任务不存在' });
        }

        // 检查所有分片是否都已上传
        if (uploadInfo.uploadedChunks.size !== uploadInfo.totalChunks) {
            return res.status(400).json({ 
                error: '还有分片未上传完成',
                uploadedChunks: uploadInfo.uploadedChunks.size,
                totalChunks: uploadInfo.totalChunks
            });
        }

        // 获取所有分片并合并
        const chunkObjects = [];
        for (let i = 0; i < uploadInfo.totalChunks; i++) {
            chunkObjects.push({
                name: `${uploadInfo.objectName}.part${i}`
            });
        }

        // 合并分片文件
        let finalObject = uploadInfo.objectName;
        if (chunkObjects.length > 1) {
            // 如果有多个分片，使用Buffer合并
            const chunks = [];
            
            // 下载所有分片到内存
            for (let i = 0; i < chunkObjects.length; i++) {
                const chunkStream = await minioClient.getObject(bucketName, chunkObjects[i].name);
                const chunks_data = [];
                
                await new Promise((resolve, reject) => {
                    chunkStream.on('data', (data) => {
                        chunks_data.push(data);
                    });
                    chunkStream.on('end', () => {
                        chunks.push(Buffer.concat(chunks_data));
                        resolve();
                    });
                    chunkStream.on('error', reject);
                });
            }
            
            // 合并所有分片
            const mergedData = Buffer.concat(chunks);
            
            // 上传合并后的文件
            await minioClient.putObject(bucketName, finalObject, mergedData);
            
            // 删除所有分片
            for (const chunk of chunkObjects) {
                try {
                    await minioClient.removeObject(bucketName, chunk.name);
                } catch (removeError) {
                    console.log(`删除分片失败: ${chunk.name}`, removeError.message);
                }
            }
        } else {
            // 只有一个分片，直接重命名（使用copyObject实现）
            await minioClient.copyObject(
                bucketName,
                finalObject,
                `/${bucketName}/${chunkObjects[0].name}`
            );
            // 删除原分片
            await minioClient.removeObject(bucketName, chunkObjects[0].name);
        }

        // 生成下载URL
        const downloadUrl = `http://${minioConfig.endPoint}:${minioConfig.port}/${bucketName}/${finalObject}`;

        // 清理上传信息
        multipartUploads.delete(uploadId);

        res.json({
            success: true,
            downloadUrl,
            fileName: uploadInfo.fileName,
            objectName: finalObject
        });
    } catch (error) {
        console.error('完成上传失败:', error);
        res.status(500).json({ error: '完成上传失败' });
    }
});

// 取消上传
app.delete('/api/upload/:uploadId', async (req, res) => {
    try {
        const { uploadId } = req.params;
        const uploadInfo = multipartUploads.get(uploadId);

        if (!uploadInfo) {
            return res.status(404).json({ error: '上传任务不存在' });
        }

        // 删除已上传的分片
        for (let i = 0; i < uploadInfo.totalChunks; i++) {
            const chunkObjectName = `${uploadInfo.objectName}.part${i}`;
            try {
                await minioClient.removeObject(bucketName, chunkObjectName);
            } catch (error) {
                // 忽略删除不存在的分片的错误
                console.log(`分片 ${i} 不存在或已删除`);
            }
        }

        // 清理上传信息
        multipartUploads.delete(uploadId);

        res.json({ success: true, message: '上传已取消' });
    } catch (error) {
        console.error('取消上传失败:', error);
        res.status(500).json({ error: '取消上传失败' });
    }
});

// 获取上传状态
app.get('/api/upload/:uploadId/status', (req, res) => {
    try {
        const { uploadId } = req.params;
        const uploadInfo = multipartUploads.get(uploadId);

        if (!uploadInfo) {
            return res.status(404).json({ error: '上传任务不存在' });
        }

        res.json({
            uploadId,
            fileName: uploadInfo.fileName,
            uploadedChunks: uploadInfo.uploadedChunks.size,
            totalChunks: uploadInfo.totalChunks,
            progress: (uploadInfo.uploadedChunks.size / uploadInfo.totalChunks) * 100
        });
    } catch (error) {
        console.error('获取上传状态失败:', error);
        res.status(500).json({ error: '获取上传状态失败' });
    }
});

// 简单的单文件上传接口（用于小文件）
app.post('/api/upload/single', upload.single('file'), async (req, res) => {
    try {
        if (!req.file) {
            return res.status(400).json({ error: '没有上传文件' });
        }

        const objectName = `${Date.now()}_${req.file.originalname}`;
        
        // 上传到MinIO
        await minioClient.putObject(bucketName, objectName, req.file.buffer);
        
        // 生成下载URL
        const downloadUrl = `http://${minioConfig.endPoint}:${minioConfig.port}/${bucketName}/${objectName}`;

        res.json({
            success: true,
            fileName: req.file.originalname,
            objectName,
            downloadUrl,
            size: req.file.size
        });
    } catch (error) {
        console.error('单文件上传失败:', error);
        res.status(500).json({ error: '文件上传失败' });
    }
});

// 列出存储桶中的文件
app.get('/api/files', async (req, res) => {
    try {
        const files = [];
        const stream = minioClient.listObjects(bucketName, '', true);
        
        for await (const obj of stream) {
            files.push({
                name: obj.name,
                size: obj.size,
                lastModified: obj.lastModified,
                url: `http://${minioConfig.endPoint}:${minioConfig.port}/${bucketName}/${obj.name}`
            });
        }

        res.json({ files });
    } catch (error) {
        console.error('获取文件列表失败:', error);
        res.status(500).json({ error: '获取文件列表失败' });
    }
});

// 删除文件
app.delete('/api/files/:objectName', async (req, res) => {
    try {
        const { objectName } = req.params;
        await minioClient.removeObject(bucketName, objectName);
        res.json({ success: true, message: '文件删除成功' });
    } catch (error) {
        console.error('删除文件失败:', error);
        res.status(500).json({ error: '删除文件失败' });
    }
});

// 错误处理中间件
app.use((error, req, res, next) => {
    console.error('服务器错误:', error);
    res.status(500).json({ error: '服务器内部错误' });
});

// 启动服务器
async function startServer() {
    try {
        await initializeBucket();
        
        app.listen(PORT, () => {
            console.log(`MinIO上传服务器运行在 http://localhost:${PORT}`);
            console.log(`MinIO服务地址: http://${minioConfig.endPoint}:${minioConfig.port}`);
        });
    } catch (error) {
        console.error('启动服务器失败:', error);
        process.exit(1);
    }
}

// 优雅关闭
process.on('SIGINT', () => {
    console.log('\n正在关闭服务器...');
    process.exit(0);
});

process.on('SIGTERM', () => {
    console.log('\n正在关闭服务器...');
    process.exit(0);
});

startServer();
