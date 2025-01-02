package gopilot

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/tidwall/gjson"
)

func models() ModelList {
	jsonStr := `{
        "object": "list",
        "data": [
            {"id": "text-search-babbage-doc-001","object": "model","created": 1651172509,"owned_by": "openai-dev"},
            {"id": "gpt-4-0613","object": "model","created": 1686588896,"owned_by": "openai"},
            {"id": "gpt-4", "object": "model", "created": 1687882411, "owned_by": "openai"},
            {"id": "babbage", "object": "model", "created": 1649358449, "owned_by": "openai"},
            {"id": "gpt-3.5-turbo-0613", "object": "model", "created": 1686587434, "owned_by": "openai"},
            {"id": "text-babbage-001", "object": "model", "created": 1649364043, "owned_by": "openai"},
            {"id": "gpt-3.5-turbo", "object": "model", "created": 1677610602, "owned_by": "openai"},
            {"id": "gpt-3.5-turbo-1106", "object": "model", "created": 1698959748, "owned_by": "system"},
            {"id": "curie-instruct-beta", "object": "model", "created": 1649364042, "owned_by": "openai"},
            {"id": "gpt-3.5-turbo-0301", "object": "model", "created": 1677649963, "owned_by": "openai"},
            {"id": "gpt-3.5-turbo-16k-0613", "object": "model", "created": 1685474247, "owned_by": "openai"},
            {"id": "text-embedding-ada-002", "object": "model", "created": 1671217299, "owned_by": "openai-internal"},
            {"id": "davinci-similarity", "object": "model", "created": 1651172509, "owned_by": "openai-dev"},
            {"id": "curie-similarity", "object": "model", "created": 1651172510, "owned_by": "openai-dev"},
            {"id": "babbage-search-document", "object": "model", "created": 1651172510, "owned_by": "openai-dev"},
            {"id": "curie-search-document", "object": "model", "created": 1651172508, "owned_by": "openai-dev"},
            {"id": "babbage-code-search-code", "object": "model", "created": 1651172509, "owned_by": "openai-dev"},
            {"id": "ada-code-search-text", "object": "model", "created": 1651172510, "owned_by": "openai-dev"},
            {"id": "text-search-curie-query-001", "object": "model", "created": 1651172509, "owned_by": "openai-dev"},
            {"id": "text-davinci-002", "object": "model", "created": 1649880484, "owned_by": "openai"},
            {"id": "ada", "object": "model", "created": 1649357491, "owned_by": "openai"},
            {"id": "text-ada-001", "object": "model", "created": 1649364042, "owned_by": "openai"},
            {"id": "ada-similarity", "object": "model", "created": 1651172507, "owned_by": "openai-dev"},
            {"id": "code-search-ada-code-001", "object": "model", "created": 1651172507, "owned_by": "openai-dev"},
            {"id": "text-similarity-ada-001", "object": "model", "created": 1651172505, "owned_by": "openai-dev"},
            {"id": "text-davinci-edit-001", "object": "model", "created": 1649809179, "owned_by": "openai"},
            {"id": "code-davinci-edit-001", "object": "model", "created": 1649880484, "owned_by": "openai"},
            {"id": "text-search-curie-doc-001", "object": "model", "created": 1651172509, "owned_by": "openai-dev"},
            {"id": "text-curie-001", "object": "model", "created": 1649364043, "owned_by": "openai"},
            {"id": "curie", "object": "model", "created": 1649359874, "owned_by": "openai"},
            {"id": "davinci", "object": "model", "created": 1649359874, "owned_by": "openai"},
            {"id": "gpt-4-0314", "object": "model", "created": 1687882410, "owned_by": "openai"}
        ]
    }`

	var modelList ModelList
	json.Unmarshal([]byte(jsonStr), &modelList)
	return modelList
}

func getAccToken(ghuToken string) (string, error) {
	var accToken = ""

	cache := cache.New(15*time.Minute, 60*time.Minute)
	cacheToken, found := cache.Get(ghuToken)
	if found {
		accToken = cacheToken.(string)
	} else {
		client := &http.Client{}
		req, err := http.NewRequest("GET", tokenUrl, nil)
		if err != nil {
			return accToken, err
		}

		headers := getHeaders(ghuToken)

		for key, value := range headers {
			req.Header.Add(key, value)
		}

		resp, err := client.Do(req)
		if err != nil {
			return accToken, err
		}
		defer resp.Body.Close()

		var reader interface{}
		switch resp.Header.Get("Content-Encoding") {
		case "gzip":
			reader, err = gzip.NewReader(resp.Body)
			if err != nil {
				return accToken, fmt.Errorf("数据解压失败")
			}
		default:
			reader = resp.Body
		}

		body, err := io.ReadAll(reader.(io.Reader))
		if err != nil {
			return accToken, fmt.Errorf("数据读取失败")
		}
		if resp.StatusCode == http.StatusOK {
			accToken = gjson.GetBytes(body, "token").String()
			if accToken == "" {
				return accToken, fmt.Errorf("acc_token 未返回")
			}
			cache.Set(ghuToken, accToken, 14*time.Minute)
		} else {
			log.Printf("获取 acc_token 请求失败：%d, %s ", resp.StatusCode, string(body))
			return accToken, fmt.Errorf("获取 acc_token 请求失败： %d", resp.StatusCode)
		}
	}
	return accToken, nil
}

func checkToken(ghuToken string) bool {
	client := &http.Client{}

	url := "https://api.github.com/user"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println(err)
		return false
	}
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("Authorization", "Bearer "+ghuToken)
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func getHeaders(ghoToken string) map[string]string {
	return map[string]string{
		"Host":          "api.github.com",
		"Authorization": "token " + ghoToken,

		"Editor-Version":        "vscode/1.85.1",
		"Editor-Plugin-Version": "copilot-chat/0.11.1",
		"User-Agent":            "GitHubCopilotChat/0.11.1",
		"Accept":                "*/*",
		"Accept-Encoding":       "gzip, deflate, br",
	}
}

func getAccHeaders(accessToken, uuid string, sessionId string, machineId string) map[string]string {
	return map[string]string{
		"Host":                   "api.githubcopilot.com",
		"Authorization":          "Bearer " + accessToken,
		"X-Request-Id":           uuid,
		"X-Github-Api-Version":   "2023-07-07",
		"Vscode-Sessionid":       sessionId,
		"Vscode-machineid":       machineId,
		"Editor-Version":         "vscode/1.85.1",
		"Editor-Plugin-Version":  "copilot-chat/0.11.1",
		"Openai-Organization":    "github-copilot",
		"Openai-Intent":          "conversation-panel",
		"Content-Type":           "application/json",
		"User-Agent":             "GitHubCopilotChat/0.11.1",
		"Copilot-Integration-Id": "vscode-chat",
		"Accept":                 "*/*",
		"Accept-Encoding":        "gzip, deflate, br",
	}
}

func getDeviceCode() (string, string, error) {
	requestUrl := "https://github.com/login/device/code"

	body := url.Values{}
	headers := map[string]string{
		"Accept": "application/json",
	}

	body.Set("client_id", client_id)
	res, err := handleRequest("POST", body, requestUrl, headers)
	deviceCode := gjson.Get(res, "device_code").String()
	userCode := gjson.Get(res, "user_code").String()

	if deviceCode == "" {
		return "", "", fmt.Errorf("device code null")
	}
	if userCode == "" {
		return "", "", fmt.Errorf("user code null")
	}
	return deviceCode, userCode, err
}

func checkUserCode(deviceCode string) (string, error) {
	requestUrl := "https://github.com/login/oauth/access_token"
	body := url.Values{}
	headers := map[string]string{
		"Accept": "application/json",
	}

	body.Set("client_id", client_id)
	body.Set("device_code", deviceCode)
	body.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	res, err := handleRequest("POST", body, requestUrl, headers)
	if err != nil {
		return "", err
	}
	token := gjson.Get(res, "access_token").String()
	return token, nil
}

func checkGhuToken(ghuToken string) string {
	requestUrl := "https://api.github.com/copilot_internal/v2/token"
	body := url.Values{}
	headers := map[string]string{
		"Authorization":         "Bearer " + ghuToken,
		"editor-version":        "JetBrains-IU/232.10203.10",
		"editor-plugin-version": "copilot-intellij/1.3.3.3572",
		"User-Agent":            "GithubCopilot/1.129.0",
		"Host":                  "api.github.com",
	}

	res, err := handleRequest("GET", body, requestUrl, headers)
	if err != nil {
		return "查询失败"
	}
	info := gjson.Get(res, "sku").String()
	if info != "" {
		return info
	} else {
		return "未订阅"
	}
}

func handleRequest(method string, body url.Values, requestUrl string, headers map[string]string) (string, error) {
	client := &http.Client{}

	req, err := http.NewRequest(method, requestUrl, bytes.NewBuffer([]byte(body.Encode())))
	if err != nil {
		return "", err
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("status code: %d, read body error", resp.StatusCode)
	}

	return string(respBody), nil
}
