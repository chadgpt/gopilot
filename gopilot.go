package gopilot

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"embed"
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

	"github.com/google/uuid"
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

var client_id = "Iv1.b507a08c87ecfe98"
var port = GetEnvOrDefault("PORT", "8081")
var ghuToken = GetEnvOrDefault("GHU_TOKEN", "")

//go:embed html/*
var embeddedFiles embed.FS

func GetEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func Run([]string) (err error) {
	log.Println("Server is running on port", port)
	log.Println("client_id:", client_id)
	log.Println("DEBUG:", os.Getenv("DEBUG") != "")

	handler := Handler()
	return http.ListenAndServe(":"+port, handler)
}

func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(models())
	})

	// /openai/deployments/aish/chat/completions?api-version=2024-04-01-preview
	mux.HandleFunc("/openai/deployments/gpt-4o/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		w.Header().Set("Connection", "keep-alive")

		requestUrl = completionsUrl
		forwardRequest(w, r)
	})

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		w.Header().Set("Connection", "keep-alive")

		requestUrl = completionsUrl
		forwardRequest(w, r)
	})

	mux.HandleFunc("/v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		w.Header().Set("Connection", "keep-alive")

		requestUrl = embeddingsUrl
		forwardRequest(w, r)
	})

	t, err := loadTemplate()
	if err != nil {
		panic(err)
	}

	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		// 获取设备授权码
		deviceCode, userCode, err := getDeviceCode()
		if err != nil {
			fmt.Fprint(w, "获取设备码失败："+err.Error())
			return
		}

		// 使用 deviceCode 和 userCode
		fmt.Println("Device Code: ", deviceCode)
		fmt.Println("User Code: ", userCode)

		t.ExecuteTemplate(w, "auth.tmpl", map[string]interface{}{
			"title":      "Get Copilot Token",
			"deviceCode": deviceCode,
			"userCode":   userCode,
		})
	})

	mux.HandleFunc("/auth/check", func(w http.ResponseWriter, r *http.Request) {
		returnData := map[string]string{
			"code": "1",
			"msg":  "",
			"data": "",
		}

		deviceCode := r.FormValue("deviceCode")
		if deviceCode == "" {
			returnData["msg"] = "device code null"
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(returnData)
			return
		}
		token, err := checkUserCode(deviceCode)
		if err != nil {
			returnData["msg"] = err.Error()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(returnData)
			return
		}
		if token == "" {
			returnData["msg"] = "token null"
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(returnData)
			return
		}
		returnData["code"] = "0"
		returnData["msg"] = "success"
		returnData["data"] = token
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(returnData)
		return
	})

	mux.HandleFunc("/auth/checkGhu", func(w http.ResponseWriter, r *http.Request) {
		returnData := map[string]string{
			"code": "1",
			"msg":  "",
			"data": "",
		}

		ghu := r.FormValue("ghu")
		if ghu == "" {
			returnData["msg"] = "ghu null"
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(returnData)
			return
		}
		if !strings.HasPrefix(ghu, "gh") {
			returnData["msg"] = "ghu 格式错误"
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(returnData)
			return
		}

		info := checkGhuToken(ghu)

		returnData["code"] = "0"
		returnData["msg"] = "success"
		returnData["data"] = info
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(returnData)
		return
	})

	return mux
}

type loggingResponseWriter struct {
	http.ResponseWriter
	logFile *os.File
}

func (lrw *loggingResponseWriter) Write(data []byte) (int, error) {
	n, err := lrw.ResponseWriter.Write(data)
	if lrw.logFile != nil {
		if _, err := lrw.logFile.Write(data); err != nil {
			log.Println("Error writing to log file:", err)
		}
	}
	return n, err
}

func forwardRequest(w http.ResponseWriter, r *http.Request) {
	var jsonBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&jsonBody); err != nil {
		http.Error(w, "Request body is missing or not in JSON format", http.StatusBadRequest)
		return
	}

	if ghuToken == "" {
		ghuToken = strings.Split(r.Header.Get("Authorization"), " ")[1]
	}

	if !strings.HasPrefix(ghuToken, "gh") {
		http.Error(w, "auth token not found", http.StatusBadRequest)
		log.Printf("token 格式错误：%s\n", ghuToken)
		return
	}

	// 检查 token 是否有效
	if !checkToken(ghuToken) {
		http.Error(w, "auth token is invalid", http.StatusBadRequest)
		log.Printf("token 无效：%s\n", ghuToken)
		return
	}
	accToken, err := getAccToken(ghuToken)
	if accToken == "" {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sessionId := fmt.Sprintf("%s%d", uuid.New().String(), time.Now().UnixNano()/int64(time.Millisecond))
	machineID := sha256.Sum256([]byte(uuid.New().String()))
	machineIDStr := hex.EncodeToString(machineID[:])
	accHeaders := getAccHeaders(accToken, uuid.New().String(), sessionId, machineIDStr)
	client := &http.Client{}

	jsonData, err := json.Marshal(jsonBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	isStream := gjson.GetBytes(jsonData, "stream").String() == "true"

	var logFile *os.File
	if os.Getenv("DEBUG") != "" {
		logFile, err = newTempfile(".")
		if err != nil {
			log.Println("Error creating log file:", err)
		} else {
			log.Println("Log file:", logFile.Name(), "isStream:", isStream)
			defer logFile.Close()

			lrw := &loggingResponseWriter{
				ResponseWriter: w,
				logFile:        logFile,
			}
			w = lrw

			body, _ := json.Marshal(jsonBody)
			logFile.Write(body)
			logFile.WriteString("\n\n")
		}
	}

	req, err := http.NewRequest("POST", requestUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, bodyString, resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if isStream {
		returnStream(w, resp)
	} else {
		returnJson(w, resp)
	}
	return
}

func returnJson(w http.ResponseWriter, resp *http.Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	body, err := io.ReadAll(resp.Body.(io.Reader))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(body)
}

func returnStream(w http.ResponseWriter, resp *http.Response) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")

	// 创建一个新的扫描器
	scanner := bufio.NewScanner(resp.Body)

	// 使用Scan方法来读取流
	for scanner.Scan() {
		line := scanner.Text()

		// 替换 "content":null 为 "content":""
		modifiedLine := strings.ReplaceAll(line, `"content":null`, `"content":""`) + "\n"

		// 将修改后的数据写入响应体
		if _, err := io.WriteString(w, modifiedLine); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if scanner.Err() != nil {
		// 处理来自扫描器的任何错误
		http.Error(w, scanner.Err().Error(), http.StatusInternalServerError)
		log.Println("Error reading from scanner:", scanner.Err())
	}
}

func loadTemplate() (*template.Template, error) {
	t := template.New("")
	files, err := embeddedFiles.ReadDir("html")
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".tmpl") {
			continue
		}
		h, err := embeddedFiles.ReadFile("html/" + file.Name())
		if err != nil {
			return nil, err
		}
		t, err = t.New(file.Name()).Parse(string(h))
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}
