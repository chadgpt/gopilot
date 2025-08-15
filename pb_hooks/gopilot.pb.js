routerAdd("POST", "/auth/checkGhu", (e) => {
  let { ghu } = e.requestInfo().body;
  let res = $http.send({
    url: "https://api.github.com/copilot_internal/v2/token",
    method: "GET",
    headers: {
      "Authorization": `Bearer ${ghu}`,
      "editor-version": "JetBrains-IU/232.10203.10",
      "editor-plugin-version": "copilot-intellij/1.3.3.3572",
      "User-Agent": "GithubCopilot/1.129.0",
      "Host": "api.github.com",
    },
    timeout: 30,
  });
  if (res.statusCode !== 200) {
    return e.json(200, {
      code: 0,
      msg: "success",
      data: `查询失败 ${res.statusCode}`,
    });
  }

  let sku = res.json?.sku;
  let info = sku ? sku : "未订阅";
  return e.json(200, { code: 0, msg: "success", data: info });
});

routerAdd("POST", "/auth/check", (e) => {
  const client_id = "Iv1.b507a08c87ecfe98";
  const grantType = "urn:ietf:params:oauth:grant-type:device_code";

  let { deviceCode } = e.requestInfo().body;
  let code = 1;
  let msg = "";
  let data = "";
  if (!deviceCode) {
    return e.json(400, { code, msg: "device code null", data });
  }
  let checkUserCode = (c) => {
    const res = $http.send({
      url: "https://github.com/login/oauth/access_token",
      method: "POST",
      body:
        `client_id=${client_id}&device_code=${deviceCode}&grant_type=${grantType}`,
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        "Accept": "application/json",
      },
      timeout: 30,
    });
    return res.json.access_token;
  };
  let token = checkUserCode(deviceCode);
  if (!token) {
    return e.json(200, { code, msg: "token null", data });
  }
  code = 0;
  return e.json(200, { code, msg: "success", data: token });
});

routerAdd("GET", "/auth", (e) => {
  const client_id = "Iv1.b507a08c87ecfe98";

  const formData = `client_id=${client_id}`;

  try {
    const res = $http.send({
      url: "https://github.com/login/device/code",
      method: "POST",
      body: formData,
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        "Accept": "application/json",
      },
      timeout: 30, // seconds
    });
    if (res.json.error) {
      return e.json(400, res.json);
    }
    let { device_code, user_code } = res.json;
    // return e.json(200, { device_code, user_code });
    const html = $template.loadFiles(
      `${__hooks}/auth.tmpl`,
    ).render({
      title: "Get Copilot Token",
      userCode: user_code,
      deviceCode: device_code,
    });

    return e.html(200, html);
  } catch (err) {
    return e.json(400, `${err}`);
  }
});
