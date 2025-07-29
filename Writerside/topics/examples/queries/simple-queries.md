# 简单查询示例

本文档展示 RAG MySQL 查询系统中各种简单查询的使用方法和示例。

## 基础数据查询

### 1. 查询所有记录

```javascript
// 查询所有用户
const result = await client.query("查询所有用户信息");

// 查询所有订单
const result = await client.query("显示所有订单");

// 查询所有产品
const result = await client.query("列出所有产品");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM users;
SELECT * FROM orders;
SELECT * FROM products;
```

### 2. 条件查询

```javascript
// 按状态查询用户
const result = await client.query("查询状态为活跃的用户");

// 按价格范围查询产品
const result = await client.query("查询价格在100到500之间的产品");

// 按时间查询订单
const result = await client.query("查询今天的订单");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM users WHERE status = 'active';
SELECT * FROM products WHERE price BETWEEN 100 AND 500;
SELECT * FROM orders WHERE DATE(created_at) = CURDATE();
```

### 3. 模糊查询

```javascript
// 按姓名模糊查询用户
const result = await client.query("查询姓名包含'张'的用户");

// 按产品名称模糊查询
const result = await client.query("查询名称包含'手机'的产品");

// 按邮箱域名查询用户
const result = await client.query("查询邮箱是gmail的用户");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM users WHERE name LIKE '%张%';
SELECT * FROM products WHERE name LIKE '%手机%';
SELECT * FROM users WHERE email LIKE '%@gmail.com';
```

## 排序和限制

### 1. 排序查询

```javascript
// 按创建时间倒序查询用户
const result = await client.query("按注册时间倒序查询用户");

// 按价格升序查询产品
const result = await client.query("按价格从低到高查询产品");

// 按订单金额降序查询
const result = await client.query("按金额从高到低查询订单");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM users ORDER BY created_at DESC;
SELECT * FROM products ORDER BY price ASC;
SELECT * FROM orders ORDER BY amount DESC;
```

### 2. 限制数量

```javascript
// 查询前10个用户
const result = await client.query("查询前10个用户", { limit: 10 });

// 查询最新的5个订单
const result = await client.query("查询最新的5个订单");

// 分页查询产品
const result = await client.query("查询产品", {
  limit: 20,
  offset: 40,
});
```

**生成的 SQL 示例：**

```sql
SELECT * FROM users LIMIT 10;
SELECT * FROM orders ORDER BY created_at DESC LIMIT 5;
SELECT * FROM products LIMIT 20 OFFSET 40;
```

## 特定字段查询

### 1. 选择特定字段

```javascript
// 只查询用户姓名和邮箱
const result = await client.query("查询用户的姓名和邮箱");

// 只查询产品名称和价格
const result = await client.query("查询产品的名称和价格");

// 查询订单号和金额
const result = await client.query("查询订单号和订单金额");
```

**生成的 SQL 示例：**

```sql
SELECT name, email FROM users;
SELECT name, price FROM products;
SELECT order_no, amount FROM orders;
```

### 2. 计算字段

```javascript
// 查询用户总数
const result = await client.query("统计用户总数");

// 查询产品平均价格
const result = await client.query("计算产品平均价格");

// 查询订单总金额
const result = await client.query("计算订单总金额");
```

**生成的 SQL 示例：**

```sql
SELECT COUNT(*) as total_users FROM users;
SELECT AVG(price) as average_price FROM products;
SELECT SUM(amount) as total_amount FROM orders;
```

## 日期和时间查询

### 1. 日期范围查询

```javascript
// 查询本月注册的用户
const result = await client.query("查询本月注册的用户");

// 查询上周的订单
const result = await client.query("查询上周的订单");

// 查询今年创建的产品
const result = await client.query("查询今年创建的产品");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM users WHERE MONTH(created_at) = MONTH(NOW()) AND YEAR(created_at) = YEAR(NOW());
SELECT * FROM orders WHERE created_at >= DATE_SUB(NOW(), INTERVAL 1 WEEK);
SELECT * FROM products WHERE YEAR(created_at) = YEAR(NOW());
```

### 2. 特定日期查询

```javascript
// 查询指定日期的订单
const result = await client.query("查询2024年1月1日的订单");

// 查询某个月的用户注册情况
const result = await client.query("查询2024年3月注册的用户");

// 查询最近7天的数据
const result = await client.query("查询最近7天的订单");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM orders WHERE DATE(created_at) = '2024-01-01';
SELECT * FROM users WHERE YEAR(created_at) = 2024 AND MONTH(created_at) = 3;
SELECT * FROM orders WHERE created_at >= DATE_SUB(NOW(), INTERVAL 7 DAY);
```

## 数值范围查询

### 1. 价格范围

```javascript
// 查询特定价格范围的产品
const result = await client.query("查询价格在50到200之间的产品");

// 查询高价产品
const result = await client.query("查询价格超过1000的产品");

// 查询低价产品
const result = await client.query("查询价格低于50的产品");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM products WHERE price BETWEEN 50 AND 200;
SELECT * FROM products WHERE price > 1000;
SELECT * FROM products WHERE price < 50;
```

### 2. 数量范围

```javascript
// 查询库存充足的产品
const result = await client.query("查询库存大于100的产品");

// 查询库存不足的产品
const result = await client.query("查询库存少于10的产品");

// 查询无库存产品
const result = await client.query("查询库存为0的产品");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM products WHERE stock_quantity > 100;
SELECT * FROM products WHERE stock_quantity < 10;
SELECT * FROM products WHERE stock_quantity = 0;
```

## 状态和枚举查询

### 1. 用户状态查询

```javascript
// 查询活跃用户
const result = await client.query("查询活跃状态的用户");

// 查询被暂停的用户
const result = await client.query("查询被暂停的用户");

// 查询非活跃用户
const result = await client.query("查询状态不是活跃的用户");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM users WHERE status = 'active';
SELECT * FROM users WHERE status = 'suspended';
SELECT * FROM users WHERE status != 'active';
```

### 2. 订单状态查询

```javascript
// 查询待支付订单
const result = await client.query("查询待支付的订单");

// 查询已完成订单
const result = await client.query("查询已交付的订单");

// 查询已取消订单
const result = await client.query("查询已取消的订单");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM orders WHERE status = 'pending';
SELECT * FROM orders WHERE status = 'delivered';
SELECT * FROM orders WHERE status = 'cancelled';
```

## 空值和非空查询

### 1. 空值查询

```javascript
// 查询没有手机号的用户
const result = await client.query("查询手机号为空的用户");

// 查询没有描述的产品
const result = await client.query("查询没有描述的产品");

// 查询没有父分类的分类
const result = await client.query("查询没有父分类的分类");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM users WHERE phone IS NULL;
SELECT * FROM products WHERE description IS NULL;
SELECT * FROM categories WHERE parent_id IS NULL;
```

### 2. 非空查询

```javascript
// 查询有手机号的用户
const result = await client.query("查询有手机号的用户");

// 查询有描述的产品
const result = await client.query("查询有产品描述的产品");

// 查询有父分类的分类
const result = await client.query("查询有父分类的分类");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM users WHERE phone IS NOT NULL;
SELECT * FROM products WHERE description IS NOT NULL;
SELECT * FROM categories WHERE parent_id IS NOT NULL;
```

## 完整示例代码

以下是一个完整的简单查询示例：

```html
<!DOCTYPE html>
<html>
  <head>
    <title>简单查询示例</title>
    <style>
      .container {
        max-width: 1000px;
        margin: 0 auto;
        padding: 20px;
      }
      .query-section {
        margin: 20px 0;
        padding: 15px;
        border: 1px solid #ddd;
      }
      .query-input {
        margin: 10px 0;
      }
      .query-input input {
        width: 70%;
        padding: 8px;
      }
      .query-input button {
        padding: 8px 15px;
        margin-left: 10px;
      }
      .results {
        margin: 15px 0;
      }
      .sql-display {
        background: #f5f5f5;
        padding: 10px;
        margin: 10px 0;
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
      .error {
        color: red;
      }
      .success {
        color: green;
      }
    </style>
  </head>
  <body>
    <div class="container">
      <h1>RAG MySQL 查询系统 - 简单查询示例</h1>

      <div class="query-section">
        <h3>基础查询</h3>
        <div class="query-input">
          <input
            type="text"
            id="basicQuery"
            placeholder="例如：查询所有用户信息"
          />
          <button onclick="executeQuery('basicQuery', 'basicResults')">
            执行查询
          </button>
        </div>
        <div id="basicResults" class="results"></div>
      </div>

      <div class="query-section">
        <h3>条件查询</h3>
        <div class="query-input">
          <input
            type="text"
            id="conditionQuery"
            placeholder="例如：查询状态为活跃的用户"
          />
          <button onclick="executeQuery('conditionQuery', 'conditionResults')">
            执行查询
          </button>
        </div>
        <div id="conditionResults" class="results"></div>
      </div>

      <div class="query-section">
        <h3>排序和限制</h3>
        <div class="query-input">
          <input
            type="text"
            id="sortQuery"
            placeholder="例如：按注册时间倒序查询前10个用户"
          />
          <button onclick="executeQuery('sortQuery', 'sortResults')">
            执行查询
          </button>
        </div>
        <div id="sortResults" class="results"></div>
      </div>

      <div class="query-section">
        <h3>统计查询</h3>
        <div class="query-input">
          <input type="text" id="statsQuery" placeholder="例如：统计用户总数" />
          <button onclick="executeQuery('statsQuery', 'statsResults')">
            执行查询
          </button>
        </div>
        <div id="statsResults" class="results"></div>
      </div>

      <div class="query-section">
        <h3>预设查询示例</h3>
        <button onclick="runPresetQuery('查询所有用户信息', 'presetResults')">
          查询所有用户
        </button>
        <button
          onclick="runPresetQuery('查询状态为活跃的用户', 'presetResults')"
        >
          查询活跃用户
        </button>
        <button onclick="runPresetQuery('统计用户总数', 'presetResults')">
          统计用户数
        </button>
        <button onclick="runPresetQuery('查询今天的订单', 'presetResults')">
          今天订单
        </button>
        <button
          onclick="runPresetQuery('按价格从低到高查询产品', 'presetResults')"
        >
          产品价格排序
        </button>
        <div id="presetResults" class="results"></div>
      </div>
    </div>

    <script>
      // 模拟 RAG 客户端
      class MockRAGClient {
        async query(queryText, options = {}) {
          // 模拟网络延迟
          await new Promise((resolve) => setTimeout(resolve, 500));

          // 模拟不同查询的响应
          return this.generateMockResponse(queryText, options);
        }

        generateMockResponse(queryText, options) {
          const responses = {
            查询所有用户信息: {
              success: true,
              sql: "SELECT * FROM users;",
              data: [
                {
                  id: 1,
                  name: "张三",
                  email: "zhang@example.com",
                  status: "active",
                  created_at: "2024-01-15",
                },
                {
                  id: 2,
                  name: "李四",
                  email: "li@example.com",
                  status: "active",
                  created_at: "2024-01-16",
                },
                {
                  id: 3,
                  name: "王五",
                  email: "wang@example.com",
                  status: "inactive",
                  created_at: "2024-01-17",
                },
              ],
              explanation:
                "查询用户表中的所有记录，返回用户的基本信息包括ID、姓名、邮箱、状态和创建时间。",
            },
            查询状态为活跃的用户: {
              success: true,
              sql: "SELECT * FROM users WHERE status = 'active';",
              data: [
                {
                  id: 1,
                  name: "张三",
                  email: "zhang@example.com",
                  status: "active",
                  created_at: "2024-01-15",
                },
                {
                  id: 2,
                  name: "李四",
                  email: "li@example.com",
                  status: "active",
                  created_at: "2024-01-16",
                },
              ],
              explanation: "查询用户表中状态为活跃(active)的用户记录。",
            },
            统计用户总数: {
              success: true,
              sql: "SELECT COUNT(*) as total_users FROM users;",
              data: [{ total_users: 156 }],
              explanation: "统计用户表中的总记录数。",
            },
            查询今天的订单: {
              success: true,
              sql: "SELECT * FROM orders WHERE DATE(created_at) = CURDATE();",
              data: [
                {
                  id: 101,
                  order_no: "ORD20240129001",
                  user_id: 1,
                  amount: 299.99,
                  status: "paid",
                  created_at: "2024-01-29 10:30:00",
                },
                {
                  id: 102,
                  order_no: "ORD20240129002",
                  user_id: 2,
                  amount: 159.5,
                  status: "pending",
                  created_at: "2024-01-29 14:20:00",
                },
              ],
              explanation: "查询今天创建的所有订单记录。",
            },
            按价格从低到高查询产品: {
              success: true,
              sql: "SELECT * FROM products ORDER BY price ASC;",
              data: [
                {
                  id: 1,
                  name: "基础套餐",
                  price: 29.99,
                  category_id: 1,
                  stock_quantity: 100,
                  status: "active",
                },
                {
                  id: 2,
                  name: "标准套餐",
                  price: 59.99,
                  category_id: 1,
                  stock_quantity: 50,
                  status: "active",
                },
                {
                  id: 3,
                  name: "高级套餐",
                  price: 99.99,
                  category_id: 1,
                  stock_quantity: 25,
                  status: "active",
                },
              ],
              explanation: "查询所有产品并按价格从低到高排序。",
            },
          };

          // 如果有预设响应，返回预设响应
          if (responses[queryText]) {
            return responses[queryText];
          }

          // 否则返回通用响应
          return {
            success: true,
            sql: `-- 为查询 "${queryText}" 生成的 SQL\nSELECT * FROM table_name WHERE condition;`,
            data: [{ message: "这是一个示例响应", query: queryText }],
            explanation: `这是对查询 "${queryText}" 的解释说明。`,
          };
        }
      }

      // 创建客户端实例
      const client = new MockRAGClient();

      // 执行查询函数
      async function executeQuery(inputId, resultId) {
        const queryText = document.getElementById(inputId).value.trim();
        if (!queryText) {
          alert("请输入查询内容");
          return;
        }

        const resultDiv = document.getElementById(resultId);
        resultDiv.innerHTML = "<div>正在执行查询...</div>";

        try {
          const result = await client.query(queryText);
          displayResult(result, resultDiv);
        } catch (error) {
          resultDiv.innerHTML = `<div class="error">查询失败: ${error.message}</div>`;
        }
      }

      // 执行预设查询
      async function runPresetQuery(queryText, resultId) {
        const resultDiv = document.getElementById(resultId);
        resultDiv.innerHTML = "<div>正在执行查询...</div>";

        try {
          const result = await client.query(queryText);
          displayResult(result, resultDiv);
        } catch (error) {
          resultDiv.innerHTML = `<div class="error">查询失败: ${error.message}</div>`;
        }
      }

      // 显示查询结果
      function displayResult(result, container) {
        let html = "";

        if (result.success) {
          html += '<div class="success">查询执行成功</div>';

          // 显示 SQL
          if (result.sql) {
            html +=
              '<div class="sql-display"><strong>生成的 SQL:</strong><br><code>' +
              result.sql +
              "</code></div>";
          }

          // 显示解释
          if (result.explanation) {
            html +=
              "<div><strong>查询解释:</strong> " +
              result.explanation +
              "</div>";
          }

          // 显示数据
          if (result.data && result.data.length > 0) {
            html += "<div><strong>查询结果:</strong></div>";
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
        } else {
          html +=
            '<div class="error">查询失败: ' +
            (result.error || "未知错误") +
            "</div>";
        }

        container.innerHTML = html;
      }
    </script>
  </body>
</html>
```

## 查询优化建议

### 1. 使用具体的查询描述

```javascript
// 推荐：具体明确
const result = await client.query("查询状态为活跃且注册时间在最近30天内的用户");

// 避免：模糊不清
const result = await client.query("查询一些用户");
```

### 2. 合理使用限制条件

```javascript
// 大数据量查询时使用限制
const result = await client.query("查询所有订单", { limit: 100 });

// 使用分页
const result = await client.query("查询用户信息", {
  limit: 20,
  offset: 40,
});
```

### 3. 启用查询解释

```javascript
// 启用解释有助于理解查询结果
const result = await client.query("统计各状态用户数量", {
  explain: true,
});
```

## 常见问题

### Q: 如何查询特定时间范围的数据？

A: 使用自然语言描述时间范围，如"查询最近 7 天的订单"、"查询 2024 年 1 月的用户"。

### Q: 如何限制查询结果数量？

A: 在查询选项中使用 `limit` 参数，或在查询文本中说明，如"查询前 10 个用户"。

### Q: 如何处理空值查询？

A: 使用"为空"、"没有"等描述，如"查询手机号为空的用户"。

### Q: 查询结果太多怎么办？

A: 使用更具体的条件或添加排序和限制，如"按注册时间倒序查询前 20 个活跃用户"。

## 下一步

- 学习 [复杂查询示例](complex-queries.md)
- 了解 [时间序列查询](time-series.md)
- 查看 [统计分析查询](analytics.md)
- 阅读 [查询优化指南](../advanced/performance.md)
