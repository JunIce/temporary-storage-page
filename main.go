package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MultipartUploadInfo 存储分片上传信息
type MultipartUploadInfo struct {
	FileName       string
	ObjectName     string
	FileSize       int64
	ChunkSize      int64
	TotalChunks    int
	UploadedChunks map[int]bool
	CreatedAt      time.Time
	mu             sync.RWMutex
}

// Config 应用配置
type Config struct {
	Port           string
	MinioEndpoint  string
	MinioPort      int
	MinioUseSSL    bool
	MinioAccessKey string
	MinioSecretKey string
	BucketName     string
}

var (
	minioClient      *minio.Client
	config           *Config
	multipartUploads = make(map[string]*MultipartUploadInfo)
	uploadsMutex     sync.RWMutex
)

// 初始化配置
func initConfig() error {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Println("未找到 .env 文件，使用环境变量")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	if minioEndpoint == "" {
		return fmt.Errorf("MINIO_ENDPOINT 环境变量未设置")
	}

	minioPortStr := os.Getenv("MINIO_PORT")
	minioPort, err := strconv.Atoi(minioPortStr)
	if err != nil {
		return fmt.Errorf("无效的 MINIO_PORT: %v", err)
	}

	minioUseSSLStr := os.Getenv("MINIO_USE_SSL")
	minioUseSSL := minioUseSSLStr == "true"

	minioAccessKey := os.Getenv("MINIO_ACCESS_KEY")
	if minioAccessKey == "" {
		return fmt.Errorf("MINIO_ACCESS_KEY 环境变量未设置")
	}

	minioSecretKey := os.Getenv("MINIO_SECRET_KEY")
	if minioSecretKey == "" {
		return fmt.Errorf("MINIO_SECRET_KEY 环境变量未设置")
	}

	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		return fmt.Errorf("BUCKET_NAME 环境变量未设置")
	}

	config = &Config{
		Port:           port,
		MinioEndpoint:  minioEndpoint,
		MinioPort:      minioPort,
		MinioUseSSL:    minioUseSSL,
		MinioAccessKey: minioAccessKey,
		MinioSecretKey: minioSecretKey,
		BucketName:     bucketName,
	}

	fmt.Printf("配置: %+v\n", config)

	return nil
}

// 初始化 MinIO 客户端
func initMinioClient() error {
	log.Printf("正在初始化 MinIO 客户端...")
	log.Printf("MinIO 端点: %s:%d", config.MinioEndpoint+":"+strconv.Itoa(config.MinioPort), config.MinioPort)

	var err error
	minioClient, err = minio.New(config.MinioEndpoint+":"+strconv.Itoa(config.MinioPort), &minio.Options{
		Creds:  credentials.NewStaticV4(config.MinioAccessKey, config.MinioSecretKey, ""),
		Secure: config.MinioUseSSL,
	})
	if err != nil {
		log.Printf("创建 MinIO 客户端失败: %v", err)
		return fmt.Errorf("创建 MinIO 客户端失败: %v", err)
	}

	log.Printf("MinIO 客户端初始化成功")
	return nil
}

// 初始化存储桶
func initializeBucket() error {
	ctx := context.Background()

	// 首先测试连接
	log.Printf("正在测试 MinIO 连接...")
	_, err := minioClient.ListBuckets(ctx)
	if err != nil {
		log.Printf("MinIO 连接测试失败: %v", err)
		return fmt.Errorf("无法连接到 MinIO 服务器: %v", err)
	}
	log.Printf("MinIO 连接测试成功")

	log.Printf("正在检查存储桶 %s 的存在性...", config.BucketName)
	exists, err := minioClient.BucketExists(ctx, config.BucketName)
	if err != nil {
		log.Printf("检查存储桶存在性时出错: %v", err)
		return fmt.Errorf("检查存储桶存在性失败: %v", err)
	}

	log.Printf("存储桶存在性检查结果: %v", exists)

	if !exists {
		log.Printf("存储桶 %s 不存在，正在创建...", config.BucketName)
		err = minioClient.MakeBucket(ctx, config.BucketName, minio.MakeBucketOptions{
			Region: "us-east-1",
		})
		if err != nil {
			log.Printf("创建存储桶失败: %v", err)
			return fmt.Errorf("创建存储桶失败: %v", err)
		}
		log.Printf("存储桶 %s 创建成功", config.BucketName)
	} else {
		log.Printf("存储桶 %s 已存在", config.BucketName)
	}

	return nil
}

// 初始化分片上传
func initMultipartUpload(c *fiber.Ctx) error {
	var request struct {
		FileName  string `json:"fileName"`
		FileSize  int64  `json:"fileSize"`
		ChunkSize int64  `json:"chunkSize"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "解析请求体失败",
		})
	}

	if request.FileName == "" || request.FileSize == 0 || request.ChunkSize == 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "缺少必要参数: fileName, fileSize, chunkSize",
		})
	}

	uploadID := uuid.New().String()
	objectName := fmt.Sprintf("%d_%s", time.Now().Unix(), request.FileName)
	totalChunks := int((request.FileSize + request.ChunkSize - 1) / request.ChunkSize)

	uploadInfo := &MultipartUploadInfo{
		FileName:       request.FileName,
		ObjectName:     objectName,
		FileSize:       request.FileSize,
		ChunkSize:      request.ChunkSize,
		TotalChunks:    totalChunks,
		UploadedChunks: make(map[int]bool),
		CreatedAt:      time.Now(),
	}

	uploadsMutex.Lock()
	multipartUploads[uploadID] = uploadInfo
	uploadsMutex.Unlock()

	return c.JSON(fiber.Map{
		"uploadId":    uploadID,
		"objectName":  objectName,
		"totalChunks": totalChunks,
	})
}

// 上传分片
func uploadChunk(c *fiber.Ctx) error {
	uploadID := c.FormValue("uploadId")
	chunkIndexStr := c.FormValue("chunkIndex")

	if uploadID == "" || chunkIndexStr == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "缺少必要参数: uploadId, chunkIndex",
		})
	}

	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "无效的 chunkIndex",
		})
	}

	uploadsMutex.RLock()
	uploadInfo, exists := multipartUploads[uploadID]
	uploadsMutex.RUnlock()

	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"error": "上传任务不存在",
		})
	}

	file, err := c.FormFile("chunk")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "获取分片文件失败",
		})
	}

	fileContent, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "打开分片文件失败",
		})
	}
	defer fileContent.Close()

	chunkObjectName := fmt.Sprintf("%s.part%d", uploadInfo.ObjectName, chunkIndex)

	ctx := context.Background()
	_, err = minioClient.PutObject(ctx, config.BucketName, chunkObjectName, fileContent, file.Size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})

	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "分片上传失败",
		})
	}

	uploadInfo.mu.Lock()
	uploadInfo.UploadedChunks[chunkIndex] = true
	uploadedCount := len(uploadInfo.UploadedChunks)
	uploadInfo.mu.Unlock()

	log.Printf("分片 %d 上传成功，对象名: %s", chunkIndex, chunkObjectName)

	return c.JSON(fiber.Map{
		"success":        true,
		"chunkIndex":     chunkIndex,
		"uploadedChunks": uploadedCount,
		"totalChunks":    uploadInfo.TotalChunks,
	})
}

// 完成分片上传
func completeMultipartUpload(c *fiber.Ctx) error {
	var request struct {
		UploadID string `json:"uploadId"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "解析请求体失败",
		})
	}

	if request.UploadID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "缺少 uploadId 参数",
		})
	}

	uploadsMutex.RLock()
	uploadInfo, exists := multipartUploads[request.UploadID]
	uploadsMutex.RUnlock()

	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"error": "上传任务不存在",
		})
	}

	uploadInfo.mu.RLock()
	uploadedCount := len(uploadInfo.UploadedChunks)
	totalChunks := uploadInfo.TotalChunks
	uploadInfo.mu.RUnlock()

	if uploadedCount != totalChunks {
		return c.Status(400).JSON(fiber.Map{
			"error":          "还有分片未上传完成",
			"uploadedChunks": uploadedCount,
			"totalChunks":    totalChunks,
		})
	}

	ctx := context.Background()
	finalObject := uploadInfo.ObjectName

	if totalChunks > 1 {
		// 多个分片，需要合并
		var chunks [][]byte
		for i := 0; i < totalChunks; i++ {
			chunkObjectName := fmt.Sprintf("%s.part%d", uploadInfo.ObjectName, i)

			obj, err := minioClient.GetObject(ctx, config.BucketName, chunkObjectName, minio.GetObjectOptions{})
			if err != nil {
				return c.Status(500).JSON(fiber.Map{
					"error": fmt.Sprintf("获取分片 %d 失败", i),
				})
			}

			chunkData, err := io.ReadAll(obj)
			obj.Close()
			if err != nil {
				return c.Status(500).JSON(fiber.Map{
					"error": fmt.Sprintf("读取分片 %d 失败", i),
				})
			}

			chunks = append(chunks, chunkData)
		}

		// 合并所有分片
		mergedData := bytes.Join(chunks, []byte{})

		// 上传合并后的文件
		_, err := minioClient.PutObject(ctx, config.BucketName, finalObject, bytes.NewReader(mergedData), int64(len(mergedData)), minio.PutObjectOptions{
			ContentType: "application/octet-stream",
		})
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "上传合并文件失败",
			})
		}

		// 删除所有分片
		for i := 0; i < totalChunks; i++ {
			chunkObjectName := fmt.Sprintf("%s.part%d", uploadInfo.ObjectName, i)
			err := minioClient.RemoveObject(ctx, config.BucketName, chunkObjectName, minio.RemoveObjectOptions{})
			if err != nil {
				log.Printf("删除分片失败: %s, %v", chunkObjectName, err)
			}
		}
	} else {
		// 只有一个分片，直接重命名
		chunkObjectName := fmt.Sprintf("%s.part0", uploadInfo.ObjectName)

		srcOpts := minio.CopySrcOptions{
			Bucket: config.BucketName,
			Object: chunkObjectName,
		}

		dstOpts := minio.CopyDestOptions{
			Bucket: config.BucketName,
			Object: finalObject,
		}

		_, err := minioClient.CopyObject(ctx, dstOpts, srcOpts)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "复制文件失败",
			})
		}

		// 删除原分片
		err = minioClient.RemoveObject(ctx, config.BucketName, chunkObjectName, minio.RemoveObjectOptions{})
		if err != nil {
			log.Printf("删除分片失败: %s, %v", chunkObjectName, err)
		}
	}

	// 生成下载URL
	downloadURL := fmt.Sprintf("http://%s:%d/%s/%s", config.MinioEndpoint, config.MinioPort, config.BucketName, finalObject)

	// 清理上传信息
	uploadsMutex.Lock()
	delete(multipartUploads, request.UploadID)
	uploadsMutex.Unlock()

	return c.JSON(fiber.Map{
		"success":     true,
		"downloadUrl": downloadURL,
		"fileName":    uploadInfo.FileName,
		"objectName":  finalObject,
	})
}

// 取消上传
func cancelUpload(c *fiber.Ctx) error {
	uploadID := c.Params("uploadId")

	uploadsMutex.RLock()
	uploadInfo, exists := multipartUploads[uploadID]
	uploadsMutex.RUnlock()

	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"error": "上传任务不存在",
		})
	}

	ctx := context.Background()

	// 删除已上传的分片
	for i := 0; i < uploadInfo.TotalChunks; i++ {
		chunkObjectName := fmt.Sprintf("%s.part%d", uploadInfo.ObjectName, i)
		err := minioClient.RemoveObject(ctx, config.BucketName, chunkObjectName, minio.RemoveObjectOptions{})
		if err != nil {
			log.Printf("分片 %d 不存在或已删除", i)
		}
	}

	// 清理上传信息
	uploadsMutex.Lock()
	delete(multipartUploads, uploadID)
	uploadsMutex.Unlock()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "上传已取消",
	})
}

// 获取上传状态
func getUploadStatus(c *fiber.Ctx) error {
	uploadID := c.Params("uploadId")

	uploadsMutex.RLock()
	uploadInfo, exists := multipartUploads[uploadID]
	uploadsMutex.RUnlock()

	if !exists {
		return c.Status(404).JSON(fiber.Map{
			"error": "上传任务不存在",
		})
	}

	uploadInfo.mu.RLock()
	uploadedCount := len(uploadInfo.UploadedChunks)
	totalChunks := uploadInfo.TotalChunks
	uploadInfo.mu.RUnlock()

	progress := float64(uploadedCount) / float64(totalChunks) * 100

	return c.JSON(fiber.Map{
		"uploadId":       uploadID,
		"fileName":       uploadInfo.FileName,
		"uploadedChunks": uploadedCount,
		"totalChunks":    totalChunks,
		"progress":       progress,
	})
}

// 单文件上传
func uploadSingle(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "没有上传文件",
		})
	}

	fileContent, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "打开文件失败",
		})
	}
	defer fileContent.Close()

	objectName := fmt.Sprintf("%d_%s", time.Now().Unix(), file.Filename)

	ctx := context.Background()
	_, err = minioClient.PutObject(ctx, config.BucketName, objectName, fileContent, file.Size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})

	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "文件上传失败",
		})
	}

	downloadURL := fmt.Sprintf("http://%s:%d/%s/%s", config.MinioEndpoint, config.MinioPort, config.BucketName, objectName)

	return c.JSON(fiber.Map{
		"success":     true,
		"fileName":    file.Filename,
		"objectName":  objectName,
		"downloadUrl": downloadURL,
		"size":        file.Size,
	})
}

// 列出文件
func listFiles(c *fiber.Ctx) error {
	ctx := context.Background()

	var files []fiber.Map

	// 创建通道来接收对象
	objectCh := minioClient.ListObjects(ctx, config.BucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": "获取文件列表失败",
			})
		}

		// 跳过分片文件
		if strings.Contains(object.Key, ".part") {
			continue
		}

		url := fmt.Sprintf("http://%s:%d/%s/%s", config.MinioEndpoint, config.MinioPort, config.BucketName, object.Key)

		files = append(files, fiber.Map{
			"name":         object.Key,
			"size":         object.Size,
			"lastModified": object.LastModified,
			"url":          url,
		})
	}

	return c.JSON(fiber.Map{
		"files": files,
	})
}

// 删除文件
func deleteFile(c *fiber.Ctx) error {
	objectName := c.Params("objectName")

	ctx := context.Background()
	err := minioClient.RemoveObject(ctx, config.BucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "删除文件失败",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "文件删除成功",
	})
}

func main() {
	// 初始化配置
	if err := initConfig(); err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	// 初始化 MinIO 客户端
	if err := initMinioClient(); err != nil {
		log.Fatalf("初始化 MinIO 客户端失败: %v", err)
	}

	// 初始化存储桶
	if err := initializeBucket(); err != nil {
		log.Fatalf("初始化存储桶失败: %v", err)
	}

	// 创建 Fiber 应用
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			log.Printf("服务器错误: %v", err)
			return c.Status(code).JSON(fiber.Map{
				"error": "服务器内部错误",
			})
		},
	})

	// 中间件配置
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// 静态文件服务 - 提供 index.html
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile(filepath.Join("index.html"))
	})

	// API 路由
	api := app.Group("/api")

	// 分片上传相关
	api.Post("/upload/init", initMultipartUpload)
	api.Post("/upload/chunk", uploadChunk)
	api.Post("/upload/complete", completeMultipartUpload)
	api.Delete("/upload/:uploadId", cancelUpload)
	api.Get("/upload/:uploadId/status", getUploadStatus)

	// 单文件上传
	api.Post("/upload/single", uploadSingle)

	// 文件管理
	api.Get("/files", listFiles)
	api.Delete("/files/:objectName", deleteFile)

	// 启动服务器
	log.Printf("MinIO上传服务器运行在 http://localhost:%s", config.Port)
	log.Printf("MinIO服务地址: http://%s:%d", config.MinioEndpoint, config.MinioPort)

	if err := app.Listen(":" + config.Port); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
