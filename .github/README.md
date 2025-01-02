# 将你的 copilot 转成 ChatGPT API（支持GPT4）

> [!IMPORTANT]
> ✨ 支持单文件部署及 docker 一键部署，简单高效
> 
> ✨ 自带获取 ghu、检测 ghu 订阅类型页面
> 
> 
> ✨ 支持中转 ghu 转 api ，即开即用
> 
## ghu token 获取

### 通过 cocoilot 获取

点击链接：[cocopilot](https://cocopilot.org/copilot/token)，根据提示拿到 ghu_xxxx 格式的 token。务必保存好，不要泄露给其他人。

### 自己部署的应用获取

地址: `http://localhost:8081/auth`

## docker 部署

``` shell


docker run -d -p 8081:8081 chadgpt/gopilot


```

## 下载程序

点击右侧的 release 下载跟你运行环境一致的可执行文件

## 运行程序

``` shell

cp .env.example .env

./gopilot

```


默认监听端口为 8081，可以在 .env 中修改

## 使用方式

可以在任意第三方客户端使用

API 域名：http://127.0.0.1:8081

API token：ghu_xxx

curl 测试

``` bash
curl --location 'http://127.0.0.1:8081/v1/chat/completions' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer ghu_xxx' \
--data '{
  "stream": true,
  "model": "gpt-4",
  "messages": [{"role": "user", "content": "hi"}]
}'
```

``` bash
curl --location 'http://127.0.0.1:8081/v1/embeddings' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer ghu_xxx' \
--data '{
  "input":["Your text string goes here"],
  "model":"text-embedding-ada-002"
  }'
```

### 程序打包

``` shell

./build.sh version

```

## 感谢以下项目，灵感来自于VV佬

[CaoYunzhou/cocopilot-gpt](https://github.com/CaoYunzhou/cocopilot-gpt)

[lvguanjun/copilot_to_chatgpt4](https://github.com/lvguanjun/copilot_to_chatgpt4)

