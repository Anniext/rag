# JavaScript 客户端集成

本文档展示如何在 JavaScript 应用中集成 RAG MySQL 查询系统，包括 Web 前端和 Node.js 后端的使用方法。

## 浏览器端集成

### 1. 基础客户端类

创建一个封装的客户端类：

```javascript
class RAGClient {
  constructor(url, options = {}) {
    this.url = url;
    this.options = {
      reconnectInterval: 5000,
      maxReconnectAttempts: 5,
      ...options,
    };

    this.ws = null;
    this.sessionId = null;
    this.messageId = 1;
    this.pendingRequests = new Map();
    this.reconnectAttempts = 0;
    this.isAuthenticated = false;

    // 事件监听器
    this.eventListeners = {
      connected: [],
      disconnected: [],
      authenticated: [],
      error: [],
      message: [],
    };
  }

  // 连接到服务器
  async connect() {
    return new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(this.url);

        this.ws.onopen = () => {
          console.log("WebSocket 连接已建立");
          this.reconnectAttempts = 0;
          this.emit("connected");
          this.negotiateProtocol().then(resolve).catch(reject);
        };

        this.ws.onmessage = (event) => {
          this.handleMessage(JSON.parse(event.data));
        };

        this.ws.onerror = (error) => {
          console.error("WebSocket 错误:", error);
          this.emit("error", error);
          reject(error);
        };

        this.ws.onclose = () => {
          console.log("WebSocket 连接已关闭");
          this.emit("disconnected");
          this.handleReconnect();
        };
      } catch (error) {
        reject(error);
      }
    });
  }

  // 协议协商
  async negotiateProtocol() {
    const response = await this.sendRequest("protocol.negotiate", {
      versions: ["1.0.0"],
    });

    if (response.result) {
      console.log("协议协商成功:", response.result.version);
      return response.result;
    } else {
      throw new Error("协议协商失败: " + response.error.message);
    }
  }

  // 用户认证
  async authenticate(token) {
    const response = await this.sendRequest("auth.authenticate", {
      token: token,
    });

    if (response.result && response.result.success) {
      this.isAuthenticated = true;
      this.emit("authenticated", response.result.user);
      return response.result;
    } else {
      throw new Error(
        "认证失败: " + (response.error ? response.error.message : "未知错误")
      );
    }
  }

  // 创建会话
  async createSession(userId) {
    const response = await this.sendRequest("session.create", {
      user_id: userId,
    });

    if (response.result) {
      this.sessionId = response.result.session_id;
      return response.result;
    } else {
      throw new Error("会话创建失败: " + response.error.message);
    }
  }

  // 执行查询
  async query(queryText, options = {}) {
    if (!this.sessionId) {
      throw new Error("会话未建立，请先创建会话");
    }

    const response = await this.sendRequest("query.execute", {
      query: queryText,
      session_id: this.sessionId,
      options: {
        limit: 20,
        explain: false,
        optimize: true,
        ...options,
      },
    });

    if (response.result) {
      return response.result;
    } else {
      throw new Error("查询失败: " + response.error.message);
    }
  }

  // 查询解释
  async explainQuery(queryText) {
    const response = await this.sendRequest("query.explain", {
      query: queryText,
    });

    if (response.result) {
      return response.result;
    } else {
      throw new Error("查询解释失败: " + response.error.message);
    }
  }

  // 获取查询建议
  async getSuggestions(partial, limit = 5) {
    const response = await this.sendRequest("query.suggest", {
      partial: partial,
      limit: limit,
    });

    if (response.result) {
      return response.result.suggestions;
    } else {
      throw new Error("获取建议失败: " + response.error.message);
    }
  }

  // 获取 Schema 信息
  async getSchemaInfo(tableName = null, includeRelationships = true) {
    const params = { include_relationships: includeRelationships };
    if (tableName) {
      params.table = tableName;
    }

    const response = await this.sendRequest("schema.info", params);

    if (response.result) {
      return response.result;
    } else {
      throw new Error("获取 Schema 失败: " + response.error.message);
    }
  }

  // 搜索 Schema
  async searchSchema(query, limit = 10) {
    const response = await this.sendRequest("schema.search", {
      query: query,
      limit: limit,
    });

    if (response.result) {
      return response.result;
    } else {
      throw new Error("Schema 搜索失败: " + response.error.message);
    }
  }

  // 发送请求
  sendRequest(method, params = {}) {
    return new Promise((resolve, reject) => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        reject(new Error("WebSocket 连接未建立"));
        return;
      }

      const id = `req-${this.messageId++}`;
      const message = {
        type: "request",
        id: id,
        method: method,
        params: params,
      };

      // 存储待处理的请求
      this.pendingRequests.set(id, { resolve, reject, timestamp: Date.now() });

      // 发送消息
      this.ws.send(JSON.stringify(message));

      // 设置超时
      setTimeout(() => {
        if (this.pendingRequests.has(id)) {
          this.pendingRequests.delete(id);
          reject(new Error("请求超时"));
        }
      }, 30000); // 30秒超时
    });
  }

  // 处理接收到的消息
  handleMessage(message) {
    this.emit("message", message);

    if (message.type === "response" && message.id) {
      const request = this.pendingRequests.get(message.id);
      if (request) {
        this.pendingRequests.delete(message.id);
        request.resolve(message);
      }
    } else if (message.type === "notification") {
      this.handleNotification(message);
    }
  }

  // 处理通知消息
  handleNotification(message) {
    switch (message.method) {
      case "query.progress":
        this.emit("queryProgress", message.params);
        break;
      case "session.expired":
        this.sessionId = null;
        this.emit("sessionExpired", message.params);
        break;
      case "system.status":
        this.emit("systemStatus", message.params);
        break;
    }
  }

  // 处理重连
  handleReconnect() {
    if (this.reconnectAttempts < this.options.maxReconnectAttempts) {
      this.reconnectAttempts++;
      console.log(
        `尝试重连 (${this.reconnectAttempts}/${this.options.maxReconnectAttempts})`
      );

      setTimeout(() => {
        this.connect().catch((error) => {
          console.error("重连失败:", error);
        });
      }, this.options.reconnectInterval);
    } else {
      console.error("达到最大重连次数，停止重连");
    }
  }

  // 事件监听
  on(event, listener) {
    if (this.eventListeners[event]) {
      this.eventListeners[event].push(listener);
    }
  }

  // 移除事件监听
  off(event, listener) {
    if (this.eventListeners[event]) {
      const index = this.eventListeners[event].indexOf(listener);
      if (index > -1) {
        this.eventListeners[event].splice(index, 1);
      }
    }
  }

  // 触发事件
  emit(event, data) {
    if (this.eventListeners[event]) {
      this.eventListeners[event].forEach((listener) => {
        try {
          listener(data);
        } catch (error) {
          console.error("事件监听器错误:", error);
        }
      });
    }
  }

  // 断开连接
  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.sessionId = null;
    this.isAuthenticated = false;
    this.pendingRequests.clear();
  }

  // 获取连接状态
  isConnected() {
    return this.ws && this.ws.readyState === WebSocket.OPEN;
  }
}
```

### 2. 使用示例

```javascript
// 创建客户端实例
const client = new RAGClient("ws://localhost:8080/mcp", {
  reconnectInterval: 3000,
  maxReconnectAttempts: 3,
});

// 设置事件监听器
client.on("connected", () => {
  console.log("客户端已连接");
});

client.on("authenticated", (user) => {
  console.log("用户已认证:", user);
});

client.on("error", (error) => {
  console.error("客户端错误:", error);
});

client.on("queryProgress", (progress) => {
  console.log("查询进度:", progress);
});

// 初始化和使用
async function initializeClient() {
  try {
    // 连接到服务器
    await client.connect();

    // 用户认证
    await client.authenticate("your-jwt-token");

    // 创建会话
    await client.createSession("user-123");

    // 执行查询
    const result = await client.query("查询所有用户信息", {
      limit: 10,
      explain: true,
    });

    console.log("查询结果:", result);

    // 获取查询建议
    const suggestions = await client.getSuggestions("查询用户");
    console.log("查询建议:", suggestions);

    // 获取 Schema 信息
    const schema = await client.getSchemaInfo("users");
    console.log("Schema 信息:", schema);
  } catch (error) {
    console.error("初始化失败:", error);
  }
}

// 启动客户端
initializeClient();
```

### 3. React 集成示例

```jsx
import React, { useState, useEffect, useCallback } from "react";

// 自定义 Hook
function useRAGClient(url, token) {
  const [client, setClient] = useState(null);
  const [isConnected, setIsConnected] = useState(false);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    const ragClient = new RAGClient(url);

    ragClient.on("connected", () => setIsConnected(true));
    ragClient.on("disconnected", () => {
      setIsConnected(false);
      setIsAuthenticated(false);
    });
    ragClient.on("authenticated", () => setIsAuthenticated(true));
    ragClient.on("error", setError);

    setClient(ragClient);

    // 自动连接和认证
    const initialize = async () => {
      try {
        await ragClient.connect();
        if (token) {
          await ragClient.authenticate(token);
          await ragClient.createSession("react-user");
        }
      } catch (err) {
        setError(err.message);
      }
    };

    initialize();

    return () => {
      ragClient.disconnect();
    };
  }, [url, token]);

  return { client, isConnected, isAuthenticated, error };
}

// 查询组件
function QueryComponent({ client }) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState(null);
  const [loading, setLoading] = useState(false);
  const [suggestions, setSuggestions] = useState([]);

  const executeQuery = useCallback(async () => {
    if (!query.trim() || !client) return;

    setLoading(true);
    try {
      const result = await client.query(query, { explain: true });
      setResults(result);
    } catch (error) {
      console.error("查询失败:", error);
      setResults({ error: error.message });
    } finally {
      setLoading(false);
    }
  }, [client, query]);

  const getSuggestions = useCallback(
    async (partial) => {
      if (!partial.trim() || !client) {
        setSuggestions([]);
        return;
      }

      try {
        const suggestions = await client.getSuggestions(partial);
        setSuggestions(suggestions);
      } catch (error) {
        console.error("获取建议失败:", error);
      }
    },
    [client]
  );

  const handleInputChange = (e) => {
    const value = e.target.value;
    setQuery(value);

    // 防抖获取建议
    clearTimeout(window.suggestionTimeout);
    window.suggestionTimeout = setTimeout(() => {
      getSuggestions(value);
    }, 300);
  };

  return (
    <div className="query-component">
      <div className="query-input">
        <input
          type="text"
          value={query}
          onChange={handleInputChange}
          placeholder="输入自然语言查询..."
          disabled={loading}
        />
        <button onClick={executeQuery} disabled={loading || !query.trim()}>
          {loading ? "查询中..." : "执行查询"}
        </button>
      </div>

      {suggestions.length > 0 && (
        <div className="suggestions">
          <h4>查询建议:</h4>
          <ul>
            {suggestions.map((suggestion, index) => (
              <li key={index} onClick={() => setQuery(suggestion)}>
                {suggestion}
              </li>
            ))}
          </ul>
        </div>
      )}

      {results && (
        <div className="results">
          {results.error ? (
            <div className="error">错误: {results.error}</div>
          ) : (
            <div>
              {results.sql && (
                <div className="sql">
                  <h4>生成的 SQL:</h4>
                  <code>{results.sql}</code>
                </div>
              )}

              {results.explanation && (
                <div className="explanation">
                  <h4>查询解释:</h4>
                  <p>{results.explanation}</p>
                </div>
              )}

              {results.data && results.data.length > 0 && (
                <div className="data">
                  <h4>查询结果:</h4>
                  <table>
                    <thead>
                      <tr>
                        {Object.keys(results.data[0]).map((key) => (
                          <th key={key}>{key}</th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {results.data.map((row, index) => (
                        <tr key={index}>
                          {Object.values(row).map((value, i) => (
                            <td key={i}>{value}</td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// 主应用组件
function App() {
  const { client, isConnected, isAuthenticated, error } = useRAGClient(
    "ws://localhost:8080/mcp",
    "your-jwt-token"
  );

  if (error) {
    return <div className="error">连接错误: {error}</div>;
  }

  if (!isConnected) {
    return <div className="loading">正在连接...</div>;
  }

  if (!isAuthenticated) {
    return <div className="loading">正在认证...</div>;
  }

  return (
    <div className="app">
      <h1>RAG MySQL 查询系统</h1>
      <QueryComponent client={client} />
    </div>
  );
}

export default App;
```

## Node.js 后端集成

### 1. 安装依赖

```bash
npm install ws jsonwebtoken
```

### 2. Node.js 客户端

```javascript
const WebSocket = require("ws");
const jwt = require("jsonwebtoken");

class RAGNodeClient {
  constructor(url, options = {}) {
    this.url = url;
    this.options = options;
    this.ws = null;
    this.sessionId = null;
    this.messageId = 1;
    this.pendingRequests = new Map();
  }

  async connect() {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.url);

      this.ws.on("open", async () => {
        console.log("连接已建立");
        try {
          await this.negotiateProtocol();
          resolve();
        } catch (error) {
          reject(error);
        }
      });

      this.ws.on("message", (data) => {
        const message = JSON.parse(data.toString());
        this.handleMessage(message);
      });

      this.ws.on("error", (error) => {
        console.error("WebSocket 错误:", error);
        reject(error);
      });

      this.ws.on("close", () => {
        console.log("连接已关闭");
      });
    });
  }

  async negotiateProtocol() {
    const response = await this.sendRequest("protocol.negotiate", {
      versions: ["1.0.0"],
    });

    if (!response.result) {
      throw new Error("协议协商失败");
    }

    return response.result;
  }

  async authenticate(token) {
    const response = await this.sendRequest("auth.authenticate", {
      token: token,
    });

    if (!response.result || !response.result.success) {
      throw new Error("认证失败");
    }

    return response.result;
  }

  async createSession(userId) {
    const response = await this.sendRequest("session.create", {
      user_id: userId,
    });

    if (!response.result) {
      throw new Error("会话创建失败");
    }

    this.sessionId = response.result.session_id;
    return response.result;
  }

  async query(queryText, options = {}) {
    const response = await this.sendRequest("query.execute", {
      query: queryText,
      session_id: this.sessionId,
      options: options,
    });

    if (!response.result) {
      throw new Error("查询失败: " + response.error.message);
    }

    return response.result;
  }

  sendRequest(method, params = {}) {
    return new Promise((resolve, reject) => {
      const id = `req-${this.messageId++}`;
      const message = {
        type: "request",
        id: id,
        method: method,
        params: params,
      };

      this.pendingRequests.set(id, { resolve, reject });
      this.ws.send(JSON.stringify(message));

      setTimeout(() => {
        if (this.pendingRequests.has(id)) {
          this.pendingRequests.delete(id);
          reject(new Error("请求超时"));
        }
      }, 30000);
    });
  }

  handleMessage(message) {
    if (message.type === "response" && message.id) {
      const request = this.pendingRequests.get(message.id);
      if (request) {
        this.pendingRequests.delete(message.id);
        request.resolve(message);
      }
    }
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
    }
  }
}

module.exports = RAGNodeClient;
```

### 3. Express.js 集成示例

```javascript
const express = require("express");
const RAGNodeClient = require("./rag-client");

const app = express();
app.use(express.json());

// 创建 RAG 客户端实例
const ragClient = new RAGNodeClient("ws://localhost:8080/mcp");

// 初始化客户端
async function initializeRAG() {
  try {
    await ragClient.connect();
    await ragClient.authenticate("server-jwt-token");
    await ragClient.createSession("server-session");
    console.log("RAG 客户端初始化成功");
  } catch (error) {
    console.error("RAG 客户端初始化失败:", error);
  }
}

// 查询接口
app.post("/api/query", async (req, res) => {
  try {
    const { query, options } = req.body;

    if (!query) {
      return res.status(400).json({ error: "查询内容不能为空" });
    }

    const result = await ragClient.query(query, options);
    res.json(result);
  } catch (error) {
    console.error("查询失败:", error);
    res.status(500).json({ error: error.message });
  }
});

// Schema 信息接口
app.get("/api/schema/:table?", async (req, res) => {
  try {
    const { table } = req.params;
    const response = await ragClient.sendRequest("schema.info", {
      table: table,
      include_relationships: true,
    });

    if (response.result) {
      res.json(response.result);
    } else {
      res.status(500).json({ error: response.error.message });
    }
  } catch (error) {
    console.error("获取 Schema 失败:", error);
    res.status(500).json({ error: error.message });
  }
});

// 查询建议接口
app.get("/api/suggestions", async (req, res) => {
  try {
    const { q, limit = 5 } = req.query;

    if (!q) {
      return res.status(400).json({ error: "查询参数不能为空" });
    }

    const response = await ragClient.sendRequest("query.suggest", {
      partial: q,
      limit: parseInt(limit),
    });

    if (response.result) {
      res.json(response.result);
    } else {
      res.status(500).json({ error: response.error.message });
    }
  } catch (error) {
    console.error("获取建议失败:", error);
    res.status(500).json({ error: error.message });
  }
});

// 启动服务器
const PORT = process.env.PORT || 3000;
app.listen(PORT, async () => {
  console.log(`服务器运行在端口 ${PORT}`);
  await initializeRAG();
});

// 优雅关闭
process.on("SIGINT", () => {
  console.log("正在关闭服务器...");
  ragClient.disconnect();
  process.exit(0);
});
```

## 错误处理和重试机制

### 1. 错误分类处理

```javascript
class RAGErrorHandler {
  static handleError(error, context = {}) {
    if (error.code) {
      switch (error.code) {
        case -31000: // 认证错误
          return this.handleAuthError(error, context);
        case -31001: // 权限错误
          return this.handlePermissionError(error, context);
        case -32002: // 超时错误
          return this.handleTimeoutError(error, context);
        case -32003: // 限流错误
          return this.handleRateLimitError(error, context);
        default:
          return this.handleGenericError(error, context);
      }
    } else {
      return this.handleGenericError(error, context);
    }
  }

  static handleAuthError(error, context) {
    console.warn("认证错误，尝试重新认证");
    // 触发重新认证流程
    if (context.client) {
      context.client.emit("authRequired");
    }
    return { retry: true, delay: 1000 };
  }

  static handlePermissionError(error, context) {
    console.error("权限不足:", error.message);
    return { retry: false, userMessage: "您没有执行此操作的权限" };
  }

  static handleTimeoutError(error, context) {
    console.warn("请求超时，准备重试");
    return { retry: true, delay: 2000, maxRetries: 3 };
  }

  static handleRateLimitError(error, context) {
    const retryAfter = error.data?.retry_after || 60;
    console.warn(`请求频率超限，${retryAfter}秒后重试`);
    return { retry: true, delay: retryAfter * 1000, maxRetries: 1 };
  }

  static handleGenericError(error, context) {
    console.error("未知错误:", error);
    return { retry: false, userMessage: "系统错误，请稍后重试" };
  }
}
```

### 2. 自动重试机制

```javascript
class RetryableRAGClient extends RAGClient {
  async queryWithRetry(queryText, options = {}, retryOptions = {}) {
    const maxRetries = retryOptions.maxRetries || 3;
    const baseDelay = retryOptions.baseDelay || 1000;

    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      try {
        return await this.query(queryText, options);
      } catch (error) {
        const errorHandling = RAGErrorHandler.handleError(error, {
          client: this,
        });

        if (!errorHandling.retry || attempt === maxRetries) {
          throw error;
        }

        const delay = errorHandling.delay || baseDelay * Math.pow(2, attempt);
        console.log(
          `查询失败，${delay}ms 后重试 (${attempt + 1}/${maxRetries})`
        );

        await new Promise((resolve) => setTimeout(resolve, delay));
      }
    }
  }
}
```

## 性能优化

### 1. 连接池管理

```javascript
class RAGConnectionPool {
  constructor(url, poolSize = 5) {
    this.url = url;
    this.poolSize = poolSize;
    this.connections = [];
    this.availableConnections = [];
    this.waitingQueue = [];
  }

  async initialize() {
    for (let i = 0; i < this.poolSize; i++) {
      const client = new RAGClient(this.url);
      await client.connect();
      this.connections.push(client);
      this.availableConnections.push(client);
    }
  }

  async getConnection() {
    return new Promise((resolve) => {
      if (this.availableConnections.length > 0) {
        const client = this.availableConnections.pop();
        resolve(client);
      } else {
        this.waitingQueue.push(resolve);
      }
    });
  }

  releaseConnection(client) {
    this.availableConnections.push(client);

    if (this.waitingQueue.length > 0) {
      const resolve = this.waitingQueue.shift();
      const nextClient = this.availableConnections.pop();
      resolve(nextClient);
    }
  }

  async executeQuery(queryText, options = {}) {
    const client = await this.getConnection();
    try {
      return await client.query(queryText, options);
    } finally {
      this.releaseConnection(client);
    }
  }
}
```

### 2. 请求缓存

```javascript
class CachedRAGClient extends RAGClient {
  constructor(url, options = {}) {
    super(url, options);
    this.cache = new Map();
    this.cacheTimeout = options.cacheTimeout || 300000; // 5分钟
  }

  async query(queryText, options = {}) {
    const cacheKey = this.getCacheKey(queryText, options);
    const cached = this.cache.get(cacheKey);

    if (cached && Date.now() - cached.timestamp < this.cacheTimeout) {
      console.log("从缓存返回结果");
      return cached.result;
    }

    const result = await super.query(queryText, options);

    // 缓存成功的查询结果
    if (result.success) {
      this.cache.set(cacheKey, {
        result: result,
        timestamp: Date.now(),
      });
    }

    return result;
  }

  getCacheKey(queryText, options) {
    return `${queryText}:${JSON.stringify(options)}`;
  }

  clearCache() {
    this.cache.clear();
  }
}
```

## 测试示例

### 1. 单元测试

```javascript
// test/rag-client.test.js
const RAGClient = require("../src/rag-client");

describe("RAGClient", () => {
  let client;

  beforeEach(() => {
    client = new RAGClient("ws://localhost:8080/mcp");
  });

  afterEach(() => {
    if (client.isConnected()) {
      client.disconnect();
    }
  });

  test("应该能够建立连接", async () => {
    await client.connect();
    expect(client.isConnected()).toBe(true);
  });

  test("应该能够执行查询", async () => {
    await client.connect();
    await client.authenticate("test-token");
    await client.createSession("test-user");

    const result = await client.query("查询用户信息");
    expect(result.success).toBe(true);
    expect(result.data).toBeDefined();
  });

  test("应该能够处理查询错误", async () => {
    await client.connect();
    await client.authenticate("test-token");
    await client.createSession("test-user");

    await expect(client.query("无效查询")).rejects.toThrow();
  });
});
```

### 2. 集成测试

```javascript
// test/integration.test.js
const RAGClient = require("../src/rag-client");

describe("RAG Integration Tests", () => {
  let client;

  beforeAll(async () => {
    client = new RAGClient("ws://localhost:8080/mcp");
    await client.connect();
    await client.authenticate(process.env.TEST_TOKEN);
    await client.createSession("integration-test");
  });

  afterAll(() => {
    client.disconnect();
  });

  test("完整的查询流程", async () => {
    // 执行查询
    const result = await client.query("查询所有用户信息", { limit: 5 });

    expect(result.success).toBe(true);
    expect(result.data).toBeInstanceOf(Array);
    expect(result.sql).toBeDefined();

    // 获取查询解释
    const explanation = await client.explainQuery("查询所有用户信息");
    expect(explanation.intent).toBeDefined();
    expect(explanation.tables).toContain("users");

    // 获取 Schema 信息
    const schema = await client.getSchemaInfo("users");
    expect(schema.table).toBeDefined();
    expect(schema.table.name).toBe("users");
  });
});
```

这个 JavaScript 客户端集成指南提供了完整的浏览器端和 Node.js 端的集成方案，包括错误处理、性能优化和测试示例。开发者可以根据自己的需求选择合适的集成方式。
