package gopilot

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/patrickmn/go-cache"
	"github.com/tidwall/gjson"
)

const tokenUrl = "https://api.github.com/copilot_internal/v2/token"
const completionsUrl = "https://api.githubcopilot.com/chat/completions"
const embeddingsUrl = "https://api.githubcopilot.com/embeddings"

var requestUrl = ""

type Model struct {
	ID      string  `json:"id"`
	Object  string  `json:"object"`
	Created int     `json:"created"`
	OwnedBy string  `json:"owned_by"`
	Root    string  `json:"root"`
	Parent  *string `json:"parent"`
}

type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

var version = "v0.6.1"
var port = "8081"
var client_id = "Iv1.b507a08c87ecfe98"

func Run([]string) (err error) {
	err = godotenv.Load()
	if err == nil {
		// 从环境变量中获取配置值
		portEnv := os.Getenv("PORT")
		if portEnv != "" {
			port = portEnv
			version = os.Getenv("VERSION")
		}
	}

	log.Printf("Server is running on port %s, version: %s\n", port, version)
	log.Printf("client_id: %s\n", client_id)

	handler := Handler()
	return http.ListenAndServe(":"+port, handler)
}

func Handler() http.Handler {
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, `
		curl --location 'http://127.0.0.1:8081/v1/chat/completions' \
		--header 'Content-Type: application/json' \
		--header 'Authorization: Bearer ghu_xxx' \
		--data '{
		  "model": "gpt-4",
		  "messages": [{"role": "user", "content": "hi"}]
		}'`)
	})

	r.GET("/v1/models", func(c *gin.Context) {
		c.JSON(http.StatusOK, models())
	})

	r.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, must-revalidate")
		c.Header("Connection", "keep-alive")

		requestUrl = completionsUrl
		forwardRequest(c)
	})

	r.POST("/v1/embeddings", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, must-revalidate")
		c.Header("Connection", "keep-alive")

		requestUrl = embeddingsUrl
		forwardRequest(c)
	})

	// 获取ghu

	t, err := loadTemplate()
	if err != nil {
		panic(err)
	}
	r.SetHTMLTemplate(t)

	r.GET("/auth", func(c *gin.Context) {
		// 获取设备授权码
		deviceCode, userCode, err := getDeviceCode()
		if err != nil {
			c.String(http.StatusOK, "获取设备码失败："+err.Error())
			return
		}

		// 使用 deviceCode 和 userCode
		fmt.Println("Device Code: ", deviceCode)
		fmt.Println("User Code: ", userCode)

		c.HTML(http.StatusOK, "/html/auth.tmpl", gin.H{
			"title":      "Get Copilot Token",
			"deviceCode": deviceCode,
			"userCode":   userCode,
		})
	})

	r.POST("/auth/check", func(c *gin.Context) {
		returnData := map[string]string{
			"code": "1",
			"msg":  "",
			"data": "",
		}

		deviceCode := c.PostForm("deviceCode")
		if deviceCode == "" {
			returnData["msg"] = "device code null"
			c.JSON(http.StatusOK, returnData)
			return
		}
		token, err := checkUserCode(deviceCode)
		if err != nil {
			returnData["msg"] = err.Error()
			c.JSON(http.StatusOK, returnData)
			return
		}
		if token == "" {
			returnData["msg"] = "token null"
			c.JSON(http.StatusOK, returnData)
			return
		}
		returnData["code"] = "0"
		returnData["msg"] = "success"
		returnData["data"] = token
		c.JSON(http.StatusOK, returnData)
		return
	})

	r.POST("/auth/checkGhu", func(c *gin.Context) {
		returnData := map[string]string{
			"code": "1",
			"msg":  "",
			"data": "",
		}

		ghu := c.PostForm("ghu")
		if ghu == "" {
			returnData["msg"] = "ghu null"
			c.JSON(http.StatusOK, returnData)
			return
		}
		if !strings.HasPrefix(ghu, "gh") {
			returnData["msg"] = "ghu 格式错误"
			c.JSON(http.StatusOK, returnData)
			return
		}

		info := checkGhuToken(ghu)

		returnData["code"] = "0"
		returnData["msg"] = "success"
		returnData["data"] = info
		c.JSON(http.StatusOK, returnData)
		return
	})

	return r
}

func forwardRequest(c *gin.Context) {
	var jsonBody map[string]interface{}
	if err := c.ShouldBindJSON(&jsonBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request body is missing or not in JSON format"})
		return
	}

	ghuToken := strings.Split(c.GetHeader("Authorization"), " ")[1]

	if !strings.HasPrefix(ghuToken, "gh") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth token not found"})
		log.Printf("token 格式错误：%s\n", ghuToken)
		return
	}

	// 检查 token 是否有效
	if !checkToken(ghuToken) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth token is invalid"})
		log.Printf("token 无效：%s\n", ghuToken)
		return
	}
	accToken, err := getAccToken(ghuToken)
	if accToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionId := fmt.Sprintf("%s%d", uuid.New().String(), time.Now().UnixNano()/int64(time.Millisecond))
	machineID := sha256.Sum256([]byte(uuid.New().String()))
	machineIDStr := hex.EncodeToString(machineID[:])
	accHeaders := getAccHeaders(accToken, uuid.New().String(), sessionId, machineIDStr)
	client := &http.Client{}

	jsonData, err := json.Marshal(jsonBody)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	isStream := gjson.GetBytes(jsonData, "stream").String() == "true"

	req, err := http.NewRequest("POST", requestUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
	}

	for key, value := range accHeaders {
		req.Header.Add(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		bodyString := string(bodyBytes)
		log.Printf("对话失败：%d, %s ", resp.StatusCode, bodyString)
		cache := cache.New(5*time.Minute, 10*time.Minute)
		cache.Delete(ghuToken)
		c.AbortWithError(resp.StatusCode, fmt.Errorf(bodyString))
		return
	}

	c.Header("Content-Type", "application/json; charset=utf-8")

	if isStream {
		returnStream(c, resp)
	} else {
		returnJson(c, resp)
	}
	return
}

func returnJson(c *gin.Context, resp *http.Response) {
	c.Header("Content-Type", "application/json; charset=utf-8")

	body, err := io.ReadAll(resp.Body.(io.Reader))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Writer.Write(body)
	return
}

func returnStream(c *gin.Context, resp *http.Response) {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")

	// 创建一个新的扫描器
	scanner := bufio.NewScanner(resp.Body)

	// 使用Scan方法来读取流
	for scanner.Scan() {
		line := scanner.Bytes()

		// 替换 "content":null 为 "content":""
		modifiedLine := bytes.Replace(line, []byte(`"content":null`), []byte(`"content":""`), -1)

		// 将修改后的数据写入响应体
		if _, err := c.Writer.Write(modifiedLine); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// 添加一个换行符
		if _, err := c.Writer.Write([]byte("\n")); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	if scanner.Err() != nil {
		// 处理来自扫描器的任何错误
		c.AbortWithError(http.StatusInternalServerError, scanner.Err())
		return
	}
	return
}

func loadTemplate() (*template.Template, error) {
	t := template.New("")
	for name, file := range Assets.Files {
		if file.IsDir() || !strings.HasSuffix(name, ".tmpl") {
			continue
		}
		h, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		t, err = t.New(name).Parse(string(h))
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}
