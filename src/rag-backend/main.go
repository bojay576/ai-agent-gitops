package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

const (
  	collectionName = "tech_docs"
	embeddingDim   = 768
)

var (
	milvusAddress  = getEnv("MILVUS_ADDRESS", "192.168.30.17:32222")
	ollamaURL          = getEnv("OLLAMA_URL", "http://172.20.10.5:11434")
	ollamaEmbeddingModel = getEnv("OLLAMA_EMBEDDING_MODEL", "nomic-embed-text")
	ollamaLLMModel     = getEnv("OLLAMA_MODEL", "qwen:7b-chat-q4_k_m")
)

// API请求和响应的JSON结构
type AskRequest struct {
	Query string `json:"query" binding:"required"`
}

type AskResponse struct {
	Answer    string   `json:"answer"`
	Sources   []string `json:"sources"`
	Error     string   `json:"error,omitempty"`
}

// Ollama API的结构体 (复用)
type OllamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}
type OllamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}
type OllamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}
type OllamaGenerateResponse struct {
	Response string `json:"response"`
}

// 全局变量，用于持有Milvus客户端
var milvusClient client.Client

func main() {
    log.Println("Starting RAG backend API...")

    var err error
    var maxRetries = 12 // 增加到12次，总共2分钟
    var retryDelay = 10 * time.Second
    var requestTimeout = 15 * time.Second // 新增：为每个Milvus调用设置15秒超时

    // --- 1. 带重试和超时的Milvus客户端初始化 ---
    for i := 0; i < maxRetries; i++ {
        // 为本次连接尝试创建一个带超时的context
        ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
        milvusClient, err = client.NewClient(ctx, client.Config{Address: milvusAddress})
        cancel() // 及时释放context资源

        if err == nil {
            log.Println("Attempting to connect to Milvus... Success!")
            break
        }
        log.Printf("Failed to connect to Milvus (attempt %d/%d): %v. Retrying...", i+1, maxRetries, err)
        time.Sleep(retryDelay)
    }
    if err != nil {
        log.Fatalf("Could not connect to Milvus after %d attempts: %v", maxRetries, err)
    }
    defer milvusClient.Close()
    
    // --- 2. 带重试和超时的Collection加载 ---
    for i := 0; i < maxRetries; i++ {
        ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
        err = milvusClient.LoadCollection(ctx, collectionName, false)
        cancel()

        if err == nil {
             log.Printf("Collection '%s' loaded successfully.", collectionName)
             break
        }
        log.Printf("Failed to load collection '%s' (attempt %d/%d): %v. Retrying...", collectionName, i+1, maxRetries, err)
        time.Sleep(retryDelay)
    }
    if err != nil {
        log.Fatalf("Could not load collection '%s' after %d attempts: %v", collectionName, err)
    }
    
    log.Println("Milvus is fully ready.")

    // 3. 初始化并启动Gin引擎
    router := gin.Default()
    router.POST("/ask", handleAsk)
    router.GET("/healthz", func(c *gin.Context) {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // 给健康检查也加上超时
        _, err := milvusClient.ListCollections(ctx)
        cancel()
        if err != nil {
            c.String(http.StatusServiceUnavailable, "Milvus not healthy")
            return
        }
        c.String(http.StatusOK, "OK")
    })
    log.Println("API server is listening on :8080")
    router.Run(":8080")
}


// API请求处理函数
func handleAsk(c *gin.Context) {
	var req AskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AskResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	log.Printf("Received query: %s", req.Query)
	ctx := context.Background()

	// --- RAG Pipeline ---

	// 1. 问题向量化
	queryEmbedding, err := getOllamaEmbedding(req.Query)
	if err != nil {
		log.Printf("Error getting query embedding: %v", err)
		c.JSON(http.StatusInternalServerError, AskResponse{Error: "Failed to process query"})
		return
	}

	// 2. 相似度搜索
	searchResults, err := searchMilvus(ctx, queryEmbedding)
	if err != nil {
		log.Printf("Error searching Milvus: %v", err)
		c.JSON(http.StatusInternalServerError, AskResponse{Error: "Failed to retrieve context"})
		return
	}
	
	if len(searchResults) == 0 {
		log.Println("No relevant documents found in Milvus.")
		c.JSON(http.StatusOK, AskResponse{Answer: "Sorry, I couldn't find any relevant information in my knowledge base.", Sources: []string{}})
		return
	}

	// 3. 构建增强的Prompt
	prompt := buildPrompt(req.Query, searchResults)

	// 4. 调用LLM生成答案
	answer, err := getOllamaGeneration(prompt)
	if err != nil {
		log.Printf("Error generating answer from LLM: %v", err)
		c.JSON(http.StatusInternalServerError, AskResponse{Error: "Failed to generate answer"})
		return
	}

	log.Printf("Generated answer: %s", answer)

	// 5. 返回响应
	c.JSON(http.StatusOK, AskResponse{
		Answer:  answer,
		Sources: searchResults,
	})
}


// (getOllamaEmbedding 函数与ingestion程序中的几乎一样)
func getOllamaEmbedding(text string) ([]float32, error) {
	// 为了简洁，你可以把这个函数和相关的结构体放到一个共享的`utils`包里
    // 但为了教学清晰，我们暂时复制过来
	reqBody, _ := json.Marshal(OllamaEmbeddingRequest{Model: ollamaEmbeddingModel, Prompt: text})
	resp, err := http.Post(ollamaURL+"/api/embeddings", "application/json", bytes.NewBuffer(reqBody))
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}
	var ollamaResp OllamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, err
	}
	return ollamaResp.Embedding, nil
}


func searchMilvus(ctx context.Context, queryVector []float32) ([]string, error) {
    // --- 核心改动：为冷启动的Milvus提供更长的超时时间 ---
    searchCtx, cancel := context.WithTimeout(ctx, 60*time.Second) // 从15秒增加到60秒
    defer cancel()
    // ---------------------------------------------------

    vec := []entity.Vector{entity.FloatVector(queryVector)}
    searchParam, _ := entity.NewIndexIvfFlatSearchParam(10)

    // 使用带超时的searchCtx
    searchResult, err := milvusClient.Search(searchCtx, collectionName, []string{}, "", []string{"text"}, vec, "embedding", entity.L2, 3, searchParam)
    if err != nil {
        // 增加一个更详细的日志，方便调试
        log.Printf("DEBUG: Milvus search failed with context. Error: %v", err)
        return nil, fmt.Errorf("milvus search failed: %w", err)
    } 

    var relevantTexts []string
    // ... (解析部分保持不变) ...
    if len(searchResult) > 0 { //...
        var textColumn *entity.ColumnVarChar
        for _, field := range searchResult[0].Fields {
            if field.Name() == "text" {
                textColumn = field.(*entity.ColumnVarChar)
                break
            }
        }
        if textColumn != nil {
            for i := 0; i < textColumn.Len(); i++ {
                text, _ := textColumn.ValueByIdx(i)
                relevantTexts = append(relevantTexts, text)
            }
        }
    }
    
    return relevantTexts, nil
}

func buildPrompt(query string, context []string) string {
	// 将所有上下文片段连接成一个字符串
	contextStr := strings.Join(context, "\n---\n")

	// 这是Prompt Engineering的关键
	prompt := fmt.Sprintf(`
You are a helpful AI assistant. Answer the user's question based on the context provided below.
If the context does not contain the answer, say "I cannot answer this question based on the provided context."

Context:
---
%s
---

User Question: %s

Answer:`, contextStr, query)
	
	return prompt
}

func getOllamaGeneration(prompt string) (string, error) {
	reqBody, _ := json.Marshal(OllamaGenerateRequest{
		Model:  ollamaLLMModel,
		Prompt: prompt,
		Stream: false,
	})

	resp, err := http.Post(ollamaURL+"/api/generate", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", err
	}

	return strings.TrimSpace(ollamaResp.Response), nil
}
