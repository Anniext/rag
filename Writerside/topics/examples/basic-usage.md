# 基本使用示例

本文档展示 RAG MySQL 查询系统的基本使用方法，包括连接建立、认证、基本查询等核心功能。

## 快速开始

### 1. 建立连接

首先建立 WebSocket 连接到 RAG 服务器：

```javascript
// 创建 WebSocket 连接
const ws = new WebSocket("ws://localhost:8080/mcp");

// 连接事件处理
ws.onopen = function () {
  console.log("连接已建立");
  // 开始协议协商
  negotiateProtocol();
};

ws.onmessage = function (event) {
  const message = JSON.parse(event.data);
  handleMessage(message);
};

ws.onerror = function (error) {
  console.error("连接错误:", error);
};

ws.onclose = function () {
  console.log("连接已关闭");
};
```

### 2. 协议协商

连接建立后，首先进行协议版本协商：

```javascript
function negotiateProtocol() {
  const negotiateMessage = {
    type: "request",
    id: "negotiate-1",
    method: "protocol.negotiate",
    params: {
      versions: ["1.0.0"],
    },
  };

  ws.send(JSON.stringify(negotiateMessage));
}

function handleMessage(message) {
  switch (message.id) {
    case "negotiate-1":
      if (message.result) {
        console.log("协议协商成功:", message.result.version);
        // 进行用户认证
        authenticate();
      } else {
        console.error("协议协商失败:", message.error);
      }
      break;
    // 其他消息处理...
  }
}
```

### 3. 用户认证

使用 JWT Token 进行用户认证：

```javascript
function authenticate() {
  const authMessage = {
    type: "request",
    id: "auth-1",
    method: "auth.authenticate",
    params: {
      token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...", // 你的 JWT Token
    },
  };

  ws.send(JSON.stringify(authMessage));
}

// 处理认证响应
function handleAuthResponse(message) {
  if (message.result && message.result.success) {
    console.log("认证成功:", message.result.user);
    // 创建会话
    createSession();
  } else {
    console.error("认证失败:", message.error);
  }
}
```

### 4. 创建会话

创建用户会话以维护查询上下文：

```javascript
let sessionId = null;

function createSession() {
  const sessionMessage = {
    type: "request",
    id: "session-create-1",
    method: "session.create",
    params: {
      user_id: "user-123",
    },
  };

  ws.send(JSON.stringify(sessionMessage));
}

// 处理会话创建响应
function handleSessionResponse(message) {
  if (message.result) {
    sessionId = message.result.session_id;
    console.log("会话创建成功:", sessionId);
    // 现在可以开始查询
    executeQuery("查询所有用户信息");
  } else {
    console.error("会话创建失败:", message.error);
  }
}
```

### 5. 执行查询

现在可以执行自然语言查询：

```javascript
function executeQuery(queryText, options = {}) {
  const queryMessage = {
    type: "request",
    id: `query-${Date.now()}`,
    method: "query.execute",
    params: {
      query: queryText,
      session_id: sessionId,
      options: {
        limit: 20,
        explain: true,
        ...options,
      },
    },
  };

  ws.send(JSON.stringify(queryMessage));
}

// 处理查询响应
function handleQueryResponse(message) {
  if (message.result && message.result.success) {
    console.log("查询成功:");
    console.log("SQL:", message.result.sql);
    console.log("数据:", message.result.data);
    console.log("解释:", message.result.explanation);

    // 显示结果
    displayResults(message.result);
  } else {
    console.error("查询失败:", message.error);
  }
}
```

## 完整示例

以下是一个完整的基本使用示例：

```html
<!DOCTYPE html>
<html>
  <head>
    <title>RAG MySQL 查询系统 - 基本示例</title>
    <style>
      body {
        font-family: Arial, sans-serif;
        margin: 20px;
      }
      .container {
        max-width: 800px;
        margin: 0 auto;
      }
      .query-box {
        margin: 20px 0;
      }
      .query-box input {
        width: 70%;
        padding: 10px;
      }
      .query-box button {
        padding: 10px 20px;
      }
      .results {
        margin: 20px 0;
      }
      .error {
        color: red;
      }
      .success {
        color: green;
      }
      table {
        width: 100%;
        border-collapse: collapse;
        margin: 10px 0;
      }
      th,
      td {
        border: 1px solid #ddd;
        padding: 8px;
        text-align: left;
      }
      th {
        background-color: #f2f2f2;
      }
    </style>
  </head>
  <body>
    <div class="container">
      <h1>RAG MySQL 查询系统 - 基本示例</h1>

      <div id="status">正在连接...</div>

      <div class="query-box">
        <input
          type="text"
          id="queryInput"
          placeholder="输入自然语言查询，例如：查询所有用户信息"
        />
        <button onclick="executeQuery()">执行查询</button>
      </div>

      <div id="results" class="results"></div>
    </div>

    <script>
      let ws = null;
      let sessionId = null;
      let messageId = 1;

      // 初始化连接
      function init() {
        ws = new WebSocket("ws://localhost:8080/mcp");

        ws.onopen = function () {
          updateStatus("连接已建立，正在协商协议...", "success");
          negotiateProtocol();
        };

        ws.onmessage = function (event) {
          const message = JSON.parse(event.data);
          handleMessage(message);
        };

        ws.onerror = function (error) {
          updateStatus("连接错误: " + error, "error");
        };

        ws.onclose = function () {
          updateStatus("连接已关闭", "error");
        };
      }

      // 协议协商
      function negotiateProtocol() {
        sendMessage({
          type: "request",
          id: "negotiate",
          method: "protocol.negotiate",
          params: { versions: ["1.0.0"] },
        });
      }

      // 用户认证
      function authenticate() {
        // 这里使用演示用的 Token，实际使用时需要从登录接口获取
        const demoToken = "demo-jwt-token-for-testing";

        sendMessage({
          type: "request",
          id: "auth",
          method: "auth.authenticate",
          params: { token: demoToken },
        });
      }

      // 创建会话
      function createSession() {
        sendMessage({
          type: "request",
          id: "session-create",
          method: "session.create",
          params: { user_id: "demo-user" },
        });
      }

      // 执行查询
      function executeQuery() {
        const queryText = document.getElementById("queryInput").value.trim();
        if (!queryText) {
          alert("请输入查询内容");
          return;
        }

        if (!sessionId) {
          alert("会话未建立，请等待初始化完成");
          return;
        }

        updateStatus("正在执行查询...", "info");

        sendMessage({
          type: "request",
          id: "query-" + messageId++,
          method: "query.execute",
          params: {
            query: queryText,
            session_id: sessionId,
            options: {
              limit: 20,
              explain: true,
              format: "json",
            },
          },
        });
      }

      // 发送消息
      function sendMessage(message) {
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify(message));
        } else {
          updateStatus("连接未建立", "error");
        }
      }

      // 处理接收到的消息
      function handleMessage(message) {
        console.log("收到消息:", message);

        switch (message.id) {
          case "negotiate":
            if (message.result) {
              updateStatus("协议协商成功，正在认证...", "success");
              authenticate();
            } else {
              updateStatus("协议协商失败: " + message.error.message, "error");
            }
            break;

          case "auth":
            if (message.result && message.result.success) {
              updateStatus("认证成功，正在创建会话...", "success");
              createSession();
            } else {
              updateStatus(
                "认证失败: " +
                  (message.error ? message.error.message : "未知错误"),
                "error"
              );
            }
            break;

          case "session-create":
            if (message.result) {
              sessionId = message.result.session_id;
              updateStatus("系统就绪，可以开始查询", "success");
            } else {
              updateStatus("会话创建失败: " + message.error.message, "error");
            }
            break;

          default:
            if (message.id && message.id.startsWith("query-")) {
              handleQueryResponse(message);
            }
            break;
        }
      }

      // 处理查询响应
      function handleQueryResponse(message) {
        if (message.result && message.result.success) {
          updateStatus("查询执行成功", "success");
          displayResults(message.result);
        } else {
          updateStatus(
            "查询失败: " + (message.error ? message.error.message : "未知错误"),
            "error"
          );
          displayError(message.error);
        }
      }

      // 显示查询结果
      function displayResults(result) {
        const resultsDiv = document.getElementById("results");

        let html = "<h3>查询结果</h3>";

        // 显示 SQL 语句
        if (result.sql) {
          html +=
            "<div><strong>生成的 SQL:</strong><br><code>" +
            result.sql +
            "</code></div>";
        }

        // 显示解释
        if (result.explanation) {
          html +=
            "<div><strong>查询解释:</strong><br>" +
            result.explanation +
            "</div>";
        }

        // 显示数据
        if (result.data && result.data.length > 0) {
          html += "<div><strong>查询数据:</strong></div>";
          html += "<table>";

          // 表头
          const headers = Object.keys(result.data[0]);
          html += "<tr>";
          headers.forEach((header) => {
            html += "<th>" + header + "</th>";
          });
          html += "</tr>";

          // 数据行
          result.data.forEach((row) => {
            html += "<tr>";
            headers.forEach((header) => {
              html += "<td>" + (row[header] || "") + "</td>";
            });
            html += "</tr>";
          });

          html += "</table>";
        } else {
          html += "<div>没有查询到数据</div>";
        }

        // 显示元数据
        if (result.metadata) {
          html += "<div><strong>执行信息:</strong><br>";
          html += "执行时间: " + result.metadata.execution_time + "<br>";
          html += "返回行数: " + result.metadata.row_count + "<br>";
          if (result.metadata.cache_hit) {
            html += "缓存命中: 是<br>";
          }
          html += "</div>";
        }

        resultsDiv.innerHTML = html;
      }

      // 显示错误信息
      function displayError(error) {
        const resultsDiv = document.getElementById("results");
        let html = "<h3>错误信息</h3>";
        html += '<div class="error">';
        html += "<strong>错误码:</strong> " + error.code + "<br>";
        html += "<strong>错误信息:</strong> " + error.message + "<br>";
        if (error.data) {
          html +=
            "<strong>详细信息:</strong> " + JSON.stringify(error.data, null, 2);
        }
        html += "</div>";
        resultsDiv.innerHTML = html;
      }

      // 更新状态显示
      function updateStatus(message, type = "info") {
        const statusDiv = document.getElementById("status");
        statusDiv.textContent = message;
        statusDiv.className = type;
      }

      // 页面加载时初始化
      window.onload = function () {
        init();
      };
    </script>
  </body>
</html>
```

## 常用查询示例

### 1. 简单查询

```javascript
// 查询所有用户
executeQuery("查询所有用户信息");

// 查询特定用户
executeQuery("查询姓名为张三的用户");

// 查询活跃用户
executeQuery("查询状态为活跃的用户");
```

### 2. 条件查询

```javascript
// 时间范围查询
executeQuery("查询今天注册的用户");

// 数量限制查询
executeQuery("查询最近10个用户", { limit: 10 });

// 排序查询
executeQuery("按注册时间倒序查询用户");
```

### 3. 统计查询

```javascript
// 计数查询
executeQuery("统计用户总数");

// 分组统计
executeQuery("按状态统计用户数量");

// 聚合查询
executeQuery("统计每个月的注册用户数");
```

### 4. 关联查询

```javascript
// 简单关联
executeQuery("查询用户及其订单信息");

// 复杂关联
executeQuery("查询用户、订单和订单详情");

// 统计关联
executeQuery("统计每个用户的订单总金额");
```

## 错误处理

### 常见错误及处理方法

```javascript
function handleQueryResponse(message) {
  if (message.error) {
    switch (message.error.code) {
      case -31000: // 认证错误
        alert("认证失败，请重新登录");
        // 重新认证逻辑
        break;

      case -31001: // 权限错误
        alert("权限不足，无法执行此查询");
        break;

      case -31300: // 查询解析错误
        alert("查询语句无法理解，请尝试更具体的描述");
        // 显示建议
        if (message.error.data && message.error.data.suggestions) {
          console.log("建议:", message.error.data.suggestions);
        }
        break;

      case -32002: // 超时错误
        alert("查询超时，请尝试简化查询或稍后重试");
        break;

      default:
        alert("查询失败: " + message.error.message);
    }
  }
}
```

## 性能优化建议

### 1. 使用会话

保持会话可以利用查询历史和上下文：

```javascript
// 在同一会话中执行相关查询
executeQuery("查询用户信息");
executeQuery("查询该用户的订单"); // 系统会理解"该用户"指的是前面查询的用户
```

### 2. 合理分页

对于大量数据，使用分页查询：

```javascript
executeQuery("查询用户信息", {
  limit: 50,
  offset: 0,
});
```

### 3. 启用缓存

重复查询会从缓存返回：

```javascript
// 第一次查询会执行数据库查询
executeQuery("统计用户总数");

// 短时间内的相同查询会从缓存返回
setTimeout(() => {
  executeQuery("统计用户总数"); // 从缓存返回
}, 1000);
```

### 4. 查询优化

启用查询优化选项：

```javascript
executeQuery("复杂的统计查询", {
  optimize: true,
  explain: true,
});
```

## 最佳实践

1. **连接管理**: 保持长连接，避免频繁建立连接
2. **错误处理**: 实现完整的错误处理逻辑
3. **会话管理**: 合理使用会话，及时清理过期会话
4. **查询优化**: 使用具体的查询描述，避免模糊查询
5. **安全考虑**: 在生产环境中使用安全的认证机制

## 下一步

- 查看 [复杂查询示例](queries/complex-queries.md)
- 学习 [客户端集成](clients/)
- 了解 [高级功能](advanced/)
- 阅读 [API 文档](../api/mcp-protocol.md)
