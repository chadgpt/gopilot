# github账号的ghu令牌获取原理

Source: https://linux.do/t/topic/21902

要逆向copilot的chat服务，首先需要获取到github账号的token，这个token一般以ghu开头，即用户级别的GitHub令牌。目前大多数人可能都是通过始皇的https://cocopilot.org/copilot/token页面获取，假如始皇的服务掉线了，有没有可能自己部署一套服务或者通过脚本来实现？

本文就是结合目前关注的一些项目，一方面分析获取ghu的原理和过程，另一方面则给出参考的示例代码，供大家参考。

事实上，获取ghu令牌只需要三个步骤：

1.获取device_code和user_code
通过以下post请求实现：

```
curl --location 'https://github.com/login/device/code' \
--header 'Accept: application/json' \
--header 'Content-Type: application/json' \
--data '{
    "client_id": "Iv1.b507a08c87ecfe98",
    "scope": "read:user"
}'
```

其中的client_id字段正是github copilot的插件应用的id，scope字段定义了请求的权限范围，表示请求读取用户信息的权限。

发出post请求后，你得到如下的响应数据：

```
{
    "device_code": "e9b9fca2f56dbdf90e9ddf562f07******",
    "user_code": "AD88-69BA",
    "verification_uri": "https://github.com/login/device",
    "expires_in": 899,
    "interval": 5
}
```

可以看到，你获取到了device_code和user_code这两个字段，后面在github登录时需要输入的就是其中的user_code。

2.登录github验证
然后就需要打开https://github.com/login/device连接，使用刚才返回的user_code激活GitHub Copilot应用。

3.查询获取访问令牌
然后就可以使用post请求携带之前获取到的device_code来获取ghu令牌了，post请求如下：

```
curl --location 'https://github.com/login/oauth/access_token' \
--header 'accept: application/json' \
--header 'content-type: application/json' \
--data '{
    "client_id": "Iv1.b507a08c87ecfe98",
    "device_code": "e9b9fca2f56dbdf90e9ddf562f07******",
    "grant_type": "urn:ietf:params:oauth:grant-type:device_code"
}'
```

发出post请求后，你会得到如下的响应数据

```
{
    "access_token": "ghu_BsvmohOSN31GWxOUHLOCclGquoeB**********",
    "token_type": "bearer",
    "scope": ""
}
```
