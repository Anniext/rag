# 复杂查询示例

本文档展示 RAG MySQL 查询系统中复杂查询的使用方法，包括多表关联、子查询、聚合分析等高级查询场景。

## 多表关联查询

### 1. 简单关联查询

```javascript
// 查询用户及其订单信息
const result = await client.query("查询用户张三的所有订单信息");

// 查询订单及其用户信息
const result = await client.query("查询订单详情包含用户姓名和邮箱");

// 查询产品及其分类信息
const result = await client.query("查询产品信息包含分类名称");
```

**生成的 SQL 示例：**

```sql
SELECT u.name, u.email, o.*
FROM users u
JOIN orders o ON u.id = o.user_id
WHERE u.name = '张三';

SELECT o.*, u.name, u.email
FROM orders o
JOIN users u ON o.user_id = u.id;

SELECT p.*, c.name as category_name
FROM products p
JOIN categories c ON p.category_id = c.id;
```

### 2. 多表关联查询

```javascript
// 查询订单、用户和订单详情
const result = await client.query("查询订单信息包含用户姓名和订单详情");

// 查询用户、订单和产品信息
const result = await client.query("查询用户张三购买的所有产品详情");

// 查询完整的订单信息链
const result = await client.query(
  "查询订单ORD001的完整信息包含用户、产品和分类"
);
```

**生成的 SQL 示例：**

```sql
SELECT o.*, u.name, u.email, oi.quantity, oi.unit_price, p.name as product_name
FROM orders o
JOIN users u ON o.user_id = u.id
JOIN order_items oi ON o.id = oi.order_id
JOIN products p ON oi.product_id = p.id;

SELECT u.name, p.name as product_name, p.price, oi.quantity, oi.total_price
FROM users u
JOIN orders o ON u.id = o.user_id
JOIN order_items oi ON o.id = oi.order_id
JOIN products p ON oi.product_id = p.id
WHERE u.name = '张三';

SELECT o.*, u.name, u.email, p.name as product_name, c.name as category_name
FROM orders o
JOIN users u ON o.user_id = u.id
JOIN order_items oi ON o.id = oi.order_id
JOIN products p ON oi.product_id = p.id
JOIN categories c ON p.category_id = c.id
WHERE o.order_no = 'ORD001';
```

### 3. 左连接查询

```javascript
// 查询所有用户及其订单数量（包括没有订单的用户）
const result = await client.query(
  "查询所有用户及其订单数量，包括没有订单的用户"
);

// 查询所有产品及其销售情况
const result = await client.query("查询所有产品的销售情况，包括未销售的产品");

// 查询分类及其产品数量
const result = await client.query("查询所有分类及其包含的产品数量");
```

**生成的 SQL 示例：**

```sql
SELECT u.*, COUNT(o.id) as order_count
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id;

SELECT p.*, COALESCE(SUM(oi.quantity), 0) as total_sold
FROM products p
LEFT JOIN order_items oi ON p.id = oi.product_id
GROUP BY p.id;

SELECT c.*, COUNT(p.id) as product_count
FROM categories c
LEFT JOIN products p ON c.id = p.category_id
GROUP BY c.id;
```

## 子查询

### 1. 标量子查询

```javascript
// 查询订单金额高于平均值的订单
const result = await client.query("查询金额高于平均订单金额的订单");

// 查询购买数量最多的用户
const result = await client.query("查询购买订单数量最多的用户");

// 查询价格高于同分类平均价格的产品
const result = await client.query("查询价格高于同分类平均价格的产品");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM orders
WHERE amount > (SELECT AVG(amount) FROM orders);

SELECT * FROM users
WHERE id = (
    SELECT user_id FROM orders
    GROUP BY user_id
    ORDER BY COUNT(*) DESC
    LIMIT 1
);

SELECT p1.* FROM products p1
WHERE p1.price > (
    SELECT AVG(p2.price)
    FROM products p2
    WHERE p2.category_id = p1.category_id
);
```

### 2. 存在性子查询

```javascript
// 查询有订单的用户
const result = await client.query("查询有订单记录的用户");

// 查询被购买过的产品
const result = await client.query("查询被购买过的产品");

// 查询有子分类的分类
const result = await client.query("查询有子分类的分类");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM users u
WHERE EXISTS (
    SELECT 1 FROM orders o WHERE o.user_id = u.id
);

SELECT * FROM products p
WHERE EXISTS (
    SELECT 1 FROM order_items oi WHERE oi.product_id = p.id
);

SELECT * FROM categories c1
WHERE EXISTS (
    SELECT 1 FROM categories c2 WHERE c2.parent_id = c1.id
);
```

### 3. IN 子查询

```javascript
// 查询购买了特定产品的用户
const result = await client.query("查询购买了手机产品的用户");

// 查询包含特定产品的订单
const result = await client.query("查询包含电子产品的订单");

// 查询活跃用户的订单
const result = await client.query("查询活跃用户的所有订单");
```

**生成的 SQL 示例：**

```sql
SELECT * FROM users
WHERE id IN (
    SELECT DISTINCT o.user_id
    FROM orders o
    JOIN order_items oi ON o.id = oi.order_id
    JOIN products p ON oi.product_id = p.id
    WHERE p.name LIKE '%手机%'
);

SELECT * FROM orders
WHERE id IN (
    SELECT DISTINCT oi.order_id
    FROM order_items oi
    JOIN products p ON oi.product_id = p.id
    JOIN categories c ON p.category_id = c.id
    WHERE c.name = '电子产品'
);

SELECT * FROM orders
WHERE user_id IN (
    SELECT id FROM users WHERE status = 'active'
);
```

## 聚合和分组查询

### 1. 基础聚合

```javascript
// 统计各状态的用户数量
const result = await client.query("统计各状态的用户数量");

// 统计各分类的产品数量和平均价格
const result = await client.query("统计各分类的产品数量和平均价格");

// 统计各用户的订单总金额
const result = await client.query("统计各用户的订单总金额");
```

**生成的 SQL 示例：**

```sql
SELECT status, COUNT(*) as user_count
FROM users
GROUP BY status;

SELECT c.name, COUNT(p.id) as product_count, AVG(p.price) as avg_price
FROM categories c
LEFT JOIN products p ON c.id = p.category_id
GROUP BY c.id, c.name;

SELECT u.name, SUM(o.amount) as total_amount
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name;
```

### 2. 条件聚合

```javascript
// 统计订单金额大于100的用户数量
const result = await client.query("统计有订单金额大于100的用户数量");

// 统计各分类中价格超过平均价格的产品数量
const result = await client.query(
  "统计各分类中价格超过该分类平均价格的产品数量"
);

// 统计最近30天各状态的订单数量
const result = await client.query("统计最近30天各状态的订单数量");
```

**生成的 SQL 示例：**

```sql
SELECT COUNT(DISTINCT u.id) as user_count
FROM users u
JOIN orders o ON u.id = o.user_id
WHERE o.amount > 100;

SELECT c.name, COUNT(*) as above_avg_count
FROM products p1
JOIN categories c ON p1.category_id = c.id
WHERE p1.price > (
    SELECT AVG(p2.price)
    FROM products p2
    WHERE p2.category_id = p1.category_id
)
GROUP BY c.id, c.name;

SELECT status, COUNT(*) as order_count
FROM orders
WHERE created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)
GROUP BY status;
```

### 3. HAVING 子句

```javascript
// 查询订单数量超过5个的用户
const result = await client.query("查询订单数量超过5个的用户");

// 查询产品数量超过10个的分类
const result = await client.query("查询产品数量超过10个的分类");

// 查询平均订单金额超过500的用户
const result = await client.query("查询平均订单金额超过500的用户");
```

**生成的 SQL 示例：**

```sql
SELECT u.name, COUNT(o.id) as order_count
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
HAVING COUNT(o.id) > 5;

SELECT c.name, COUNT(p.id) as product_count
FROM categories c
JOIN products p ON c.id = p.category_id
GROUP BY c.id, c.name
HAVING COUNT(p.id) > 10;

SELECT u.name, AVG(o.amount) as avg_amount
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
HAVING AVG(o.amount) > 500;
```

## 窗口函数查询

### 1. 排名函数

```javascript
// 查询各分类中价格排名前3的产品
const result = await client.query("查询各分类中价格最高的前3个产品");

// 查询用户订单金额排名
const result = await client.query("查询用户按订单总金额的排名");

// 查询产品销量排名
const result = await client.query("查询产品按销量的排名");
```

**生成的 SQL 示例：**

```sql
SELECT *
FROM (
    SELECT p.*, c.name as category_name,
           ROW_NUMBER() OVER (PARTITION BY p.category_id ORDER BY p.price DESC) as price_rank
    FROM products p
    JOIN categories c ON p.category_id = c.id
) ranked
WHERE price_rank <= 3;

SELECT u.name, SUM(o.amount) as total_amount,
       RANK() OVER (ORDER BY SUM(o.amount) DESC) as amount_rank
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name;

SELECT p.name, SUM(oi.quantity) as total_sold,
       DENSE_RANK() OVER (ORDER BY SUM(oi.quantity) DESC) as sales_rank
FROM products p
JOIN order_items oi ON p.id = oi.product_id
GROUP BY p.id, p.name;
```

### 2. 聚合窗口函数

```javascript
// 查询用户订单及累计金额
const result = await client.query("查询用户订单按时间排序的累计金额");

// 查询产品销售的移动平均
const result = await client.query("查询产品最近3个月的销售移动平均");

// 查询订单金额的百分位数
const result = await client.query("查询订单金额的分布百分位数");
```

**生成的 SQL 示例：**

```sql
SELECT u.name, o.created_at, o.amount,
       SUM(o.amount) OVER (PARTITION BY u.id ORDER BY o.created_at) as cumulative_amount
FROM users u
JOIN orders o ON u.id = o.user_id
ORDER BY u.id, o.created_at;

SELECT p.name, DATE(o.created_at) as order_date,
       SUM(oi.quantity) as daily_sales,
       AVG(SUM(oi.quantity)) OVER (
           PARTITION BY p.id
           ORDER BY DATE(o.created_at)
           ROWS BETWEEN 2 PRECEDING AND CURRENT ROW
       ) as moving_avg
FROM products p
JOIN order_items oi ON p.id = oi.product_id
JOIN orders o ON oi.order_id = o.id
GROUP BY p.id, p.name, DATE(o.created_at);

SELECT amount,
       PERCENT_RANK() OVER (ORDER BY amount) as percentile
FROM orders
ORDER BY amount;
```

## 条件逻辑查询

### 1. CASE WHEN 语句

```javascript
// 根据订单金额分级统计
const result = await client.query("按订单金额分为小额、中额、大额统计数量");

// 根据用户注册时间分组
const result = await client.query("按用户注册时间分为新用户、老用户统计");

// 根据产品库存状态分类
const result = await client.query("按产品库存分为充足、不足、缺货统计");
```

**生成的 SQL 示例：**

```sql
SELECT
    CASE
        WHEN amount < 100 THEN '小额订单'
        WHEN amount BETWEEN 100 AND 500 THEN '中额订单'
        ELSE '大额订单'
    END as order_type,
    COUNT(*) as order_count
FROM orders
GROUP BY
    CASE
        WHEN amount < 100 THEN '小额订单'
        WHEN amount BETWEEN 100 AND 500 THEN '中额订单'
        ELSE '大额订单'
    END;

SELECT
    CASE
        WHEN created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY) THEN '新用户'
        ELSE '老用户'
    END as user_type,
    COUNT(*) as user_count
FROM users
GROUP BY
    CASE
        WHEN created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY) THEN '新用户'
        ELSE '老用户'
    END;

SELECT
    CASE
        WHEN stock_quantity > 50 THEN '库存充足'
        WHEN stock_quantity > 0 THEN '库存不足'
        ELSE '缺货'
    END as stock_status,
    COUNT(*) as product_count
FROM products
GROUP BY
    CASE
        WHEN stock_quantity > 50 THEN '库存充足'
        WHEN stock_quantity > 0 THEN '库存不足'
        ELSE '缺货'
    END;
```

### 2. 条件聚合

```javascript
// 统计各用户的有效订单和取消订单数量
const result = await client.query("统计各用户的有效订单和取消订单数量");

// 统计各分类的在售和下架产品数量
const result = await client.query("统计各分类的在售和下架产品数量");

// 统计各月的新增和流失用户数量
const result = await client.query("统计各月的新增用户数量");
```

**生成的 SQL 示例：**

```sql
SELECT u.name,
       SUM(CASE WHEN o.status != 'cancelled' THEN 1 ELSE 0 END) as valid_orders,
       SUM(CASE WHEN o.status = 'cancelled' THEN 1 ELSE 0 END) as cancelled_orders
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name;

SELECT c.name,
       SUM(CASE WHEN p.status = 'active' THEN 1 ELSE 0 END) as active_products,
       SUM(CASE WHEN p.status = 'inactive' THEN 1 ELSE 0 END) as inactive_products
FROM categories c
LEFT JOIN products p ON c.id = p.category_id
GROUP BY c.id, c.name;

SELECT DATE_FORMAT(created_at, '%Y-%m') as month,
       COUNT(*) as new_users
FROM users
GROUP BY DATE_FORMAT(created_at, '%Y-%m')
ORDER BY month;
```

## 时间序列分析

### 1. 时间分组统计

```javascript
// 按月统计订单数量和金额
const result = await client.query("按月统计订单数量和总金额");

// 按周统计用户注册数量
const result = await client.query("按周统计用户注册数量");

// 按小时统计订单分布
const result = await client.query("按小时统计订单创建分布");
```

**生成的 SQL 示例：**

```sql
SELECT DATE_FORMAT(created_at, '%Y-%m') as month,
       COUNT(*) as order_count,
       SUM(amount) as total_amount
FROM orders
GROUP BY DATE_FORMAT(created_at, '%Y-%m')
ORDER BY month;

SELECT YEARWEEK(created_at) as week,
       COUNT(*) as user_count
FROM users
GROUP BY YEARWEEK(created_at)
ORDER BY week;

SELECT HOUR(created_at) as hour,
       COUNT(*) as order_count
FROM orders
GROUP BY HOUR(created_at)
ORDER BY hour;
```

### 2. 同比环比分析

```javascript
// 查询各月订单数量的环比增长
const result = await client.query("查询各月订单数量的环比增长率");

// 查询用户注册的同比增长
const result = await client.query("查询用户注册的同比增长情况");

// 查询产品销售的季度对比
const result = await client.query("查询产品销售的季度对比");
```

**生成的 SQL 示例：**

```sql
SELECT month,
       order_count,
       LAG(order_count) OVER (ORDER BY month) as prev_month_count,
       ROUND((order_count - LAG(order_count) OVER (ORDER BY month)) * 100.0 /
             LAG(order_count) OVER (ORDER BY month), 2) as growth_rate
FROM (
    SELECT DATE_FORMAT(created_at, '%Y-%m') as month,
           COUNT(*) as order_count
    FROM orders
    GROUP BY DATE_FORMAT(created_at, '%Y-%m')
) monthly_stats
ORDER BY month;

SELECT YEAR(created_at) as year,
       MONTH(created_at) as month,
       COUNT(*) as current_year_count,
       LAG(COUNT(*), 12) OVER (ORDER BY YEAR(created_at), MONTH(created_at)) as last_year_count
FROM users
GROUP BY YEAR(created_at), MONTH(created_at)
ORDER BY year, month;

SELECT p.name,
       QUARTER(o.created_at) as quarter,
       SUM(oi.quantity) as quarterly_sales
FROM products p
JOIN order_items oi ON p.id = oi.product_id
JOIN orders o ON oi.order_id = o.id
GROUP BY p.id, p.name, QUARTER(o.created_at)
ORDER BY p.name, quarter;
```

## 完整示例应用

以下是一个复杂查询的完整示例应用：

```html
<!DOCTYPE html>
<html>
  <head>
    <title>复杂查询示例</title>
    <style>
      .container {
        max-width: 1200px;
        margin: 0 auto;
        padding: 20px;
      }
      .query-section {
        margin: 20px 0;
        padding: 15px;
        border: 1px solid #ddd;
        border-radius: 5px;
      }
      .query-tabs {
        display: flex;
        margin-bottom: 15px;
      }
      .tab {
        padding: 10px 20px;
        background: #f5f5f5;
        border: 1px solid #ddd;
        cursor: pointer;
      }
      .tab.active {
        background: #007bff;
        color: white;
      }
      .query-content {
        display: none;
      }
      .query-content.active {
        display: block;
      }
      .query-input {
        margin: 10px 0;
      }
      .query-input textarea {
        width: 100%;
        height: 60px;
        padding: 8px;
      }
      .query-input button {
        padding: 10px 20px;
        margin: 5px;
      }
      .results {
        margin: 15px 0;
        max-height: 400px;
        overflow-y: auto;
      }
      .sql-display {
        background: #f8f9fa;
        padding: 15px;
        margin: 10px 0;
        border-radius: 5px;
      }
      .explanation {
        background: #e7f3ff;
        padding: 10px;
        margin: 10px 0;
        border-radius: 5px;
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
        font-size: 12px;
      }
      th {
        background-color: #f2f2f2;
      }
      .error {
        color: red;
        background: #ffe6e6;
        padding: 10px;
        border-radius: 5px;
      }
      .success {
        color: green;
      }
      .preset-queries {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
        gap: 10px;
        margin: 15px 0;
      }
      .preset-query {
        padding: 10px;
        background: #f8f9fa;
        border: 1px solid #ddd;
        border-radius: 5px;
        cursor: pointer;
      }
      .preset-query:hover {
        background: #e9ecef;
      }
    </style>
  </head>
  <body>
    <div class="container">
      <h1>RAG MySQL 查询系统 - 复杂查询示例</h1>

      <div class="query-section">
        <div class="query-tabs">
          <div class="tab active" onclick="showTab('join')">关联查询</div>
          <div class="tab" onclick="showTab('subquery')">子查询</div>
          <div class="tab" onclick="showTab('aggregate')">聚合分析</div>
          <div class="tab" onclick="showTab('window')">窗口函数</div>
          <div class="tab" onclick="showTab('timeseries')">时间序列</div>
        </div>

        <!-- 关联查询 -->
        <div id="join" class="query-content active">
          <h3>多表关联查询</h3>
          <div class="preset-queries">
            <div
              class="preset-query"
              onclick="runPresetQuery('查询用户及其订单信息包含产品详情', 'joinResults')"
            >
              用户订单产品关联查询
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('查询各分类的产品数量和平均价格', 'joinResults')"
            >
              分类产品统计
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('查询所有用户及其订单数量包括没有订单的用户', 'joinResults')"
            >
              用户订单数量（含零订单）
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('查询订单详情包含用户姓名和产品信息', 'joinResults')"
            >
              完整订单信息
            </div>
          </div>
          <div class="query-input">
            <textarea
              id="joinQuery"
              placeholder="输入关联查询，例如：查询用户张三的所有订单及产品详情"
            ></textarea>
            <button onclick="executeQuery('joinQuery', 'joinResults')">
              执行查询
            </button>
          </div>
          <div id="joinResults" class="results"></div>
        </div>

        <!-- 子查询 -->
        <div id="subquery" class="query-content">
          <h3>子查询示例</h3>
          <div class="preset-queries">
            <div
              class="preset-query"
              onclick="runPresetQuery('查询订单金额高于平均订单金额的订单', 'subqueryResults')"
            >
              高于平均金额的订单
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('查询购买了手机产品的用户', 'subqueryResults')"
            >
              购买特定产品的用户
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('查询有订单记录的用户', 'subqueryResults')"
            >
              有订单的用户
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('查询价格高于同分类平均价格的产品', 'subqueryResults')"
            >
              高于分类平均价的产品
            </div>
          </div>
          <div class="query-input">
            <textarea
              id="subqueryQuery"
              placeholder="输入子查询，例如：查询购买数量最多的用户"
            ></textarea>
            <button onclick="executeQuery('subqueryQuery', 'subqueryResults')">
              执行查询
            </button>
          </div>
          <div id="subqueryResults" class="results"></div>
        </div>

        <!-- 聚合分析 -->
        <div id="aggregate" class="query-content">
          <h3>聚合分析查询</h3>
          <div class="preset-queries">
            <div
              class="preset-query"
              onclick="runPresetQuery('统计各状态的用户数量', 'aggregateResults')"
            >
              用户状态统计
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('统计各用户的订单总金额', 'aggregateResults')"
            >
              用户消费统计
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('查询订单数量超过5个的用户', 'aggregateResults')"
            >
              高频用户查询
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('按订单金额分为小额中额大额统计数量', 'aggregateResults')"
            >
              订单金额分级统计
            </div>
          </div>
          <div class="query-input">
            <textarea
              id="aggregateQuery"
              placeholder="输入聚合查询，例如：统计各分类的产品数量和平均价格"
            ></textarea>
            <button
              onclick="executeQuery('aggregateQuery', 'aggregateResults')"
            >
              执行查询
            </button>
          </div>
          <div id="aggregateResults" class="results"></div>
        </div>

        <!-- 窗口函数 -->
        <div id="window" class="query-content">
          <h3>窗口函数查询</h3>
          <div class="preset-queries">
            <div
              class="preset-query"
              onclick="runPresetQuery('查询各分类中价格最高的前3个产品', 'windowResults')"
            >
              分类价格排名
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('查询用户按订单总金额的排名', 'windowResults')"
            >
              用户消费排名
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('查询用户订单按时间排序的累计金额', 'windowResults')"
            >
              订单累计金额
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('查询产品按销量的排名', 'windowResults')"
            >
              产品销量排名
            </div>
          </div>
          <div class="query-input">
            <textarea
              id="windowQuery"
              placeholder="输入窗口函数查询，例如：查询各用户的订单排名"
            ></textarea>
            <button onclick="executeQuery('windowQuery', 'windowResults')">
              执行查询
            </button>
          </div>
          <div id="windowResults" class="results"></div>
        </div>

        <!-- 时间序列 -->
        <div id="timeseries" class="query-content">
          <h3>时间序列分析</h3>
          <div class="preset-queries">
            <div
              class="preset-query"
              onclick="runPresetQuery('按月统计订单数量和总金额', 'timeseriesResults')"
            >
              月度订单统计
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('按周统计用户注册数量', 'timeseriesResults')"
            >
              周度用户注册
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('查询各月订单数量的环比增长率', 'timeseriesResults')"
            >
              订单环比增长
            </div>
            <div
              class="preset-query"
              onclick="runPresetQuery('按小时统计订单创建分布', 'timeseriesResults')"
            >
              小时订单分布
            </div>
          </div>
          <div class="query-input">
            <textarea
              id="timeseriesQuery"
              placeholder="输入时间序列查询，例如：按季度统计产品销售情况"
            ></textarea>
            <button
              onclick="executeQuery('timeseriesQuery', 'timeseriesResults')"
            >
              执行查询
            </button>
          </div>
          <div id="timeseriesResults" class="results"></div>
        </div>
      </div>
    </div>

    <script>
      // 模拟复杂查询的 RAG 客户端
      class ComplexRAGClient {
        async query(queryText, options = {}) {
          await new Promise((resolve) => setTimeout(resolve, 800));
          return this.generateComplexResponse(queryText, options);
        }

        generateComplexResponse(queryText, options) {
          const responses = {
            查询用户及其订单信息包含产品详情: {
              success: true,
              sql: `SELECT u.name, u.email, o.order_no, o.amount, p.name as product_name, oi.quantity
FROM users u
JOIN orders o ON u.id = o.user_id
JOIN order_items oi ON o.id = oi.order_id
JOIN products p ON oi.product_id = p.id
ORDER BY u.name, o.created_at;`,
              data: [
                {
                  name: "张三",
                  email: "zhang@example.com",
                  order_no: "ORD001",
                  amount: 299.99,
                  product_name: "智能手机",
                  quantity: 1,
                },
                {
                  name: "张三",
                  email: "zhang@example.com",
                  order_no: "ORD001",
                  amount: 299.99,
                  product_name: "手机壳",
                  quantity: 2,
                },
                {
                  name: "李四",
                  email: "li@example.com",
                  order_no: "ORD002",
                  amount: 159.5,
                  product_name: "蓝牙耳机",
                  quantity: 1,
                },
              ],
              explanation:
                "通过多表关联查询，获取用户、订单和产品的完整信息，展示用户的购买详情。",
            },
            查询各分类的产品数量和平均价格: {
              success: true,
              sql: `SELECT c.name, COUNT(p.id) as product_count, AVG(p.price) as avg_price
FROM categories c
LEFT JOIN products p ON c.id = p.category_id
GROUP BY c.id, c.name;`,
              data: [
                { name: "电子产品", product_count: 25, avg_price: 456.78 },
                { name: "服装", product_count: 18, avg_price: 89.32 },
                { name: "家居用品", product_count: 12, avg_price: 234.56 },
              ],
              explanation:
                "统计各产品分类的产品数量和平均价格，使用LEFT JOIN确保包含没有产品的分类。",
            },
            查询订单金额高于平均订单金额的订单: {
              success: true,
              sql: `SELECT * FROM orders 
WHERE amount > (SELECT AVG(amount) FROM orders);`,
              data: [
                {
                  id: 101,
                  order_no: "ORD001",
                  user_id: 1,
                  amount: 599.99,
                  status: "paid",
                  created_at: "2024-01-29",
                },
                {
                  id: 103,
                  order_no: "ORD003",
                  user_id: 3,
                  amount: 899.5,
                  status: "delivered",
                  created_at: "2024-01-28",
                },
              ],
              explanation:
                "使用子查询计算平均订单金额，然后筛选出高于平均值的订单。",
            },
            统计各状态的用户数量: {
              success: true,
              sql: `SELECT status, COUNT(*) as user_count
FROM users
GROUP BY status;`,
              data: [
                { status: "active", user_count: 142 },
                { status: "inactive", user_count: 23 },
                { status: "suspended", user_count: 5 },
              ],
              explanation: "按用户状态分组统计，了解用户分布情况。",
            },
            查询各分类中价格最高的前3个产品: {
              success: true,
              sql: `SELECT *
FROM (
    SELECT p.*, c.name as category_name,
           ROW_NUMBER() OVER (PARTITION BY p.category_id ORDER BY p.price DESC) as price_rank
    FROM products p
    JOIN categories c ON p.category_id = c.id
) ranked
WHERE price_rank <= 3;`,
              data: [
                {
                  name: "iPhone 15 Pro",
                  price: 7999.0,
                  category_name: "电子产品",
                  price_rank: 1,
                },
                {
                  name: "MacBook Pro",
                  price: 12999.0,
                  category_name: "电子产品",
                  price_rank: 2,
                },
                {
                  name: "iPad Pro",
                  price: 6999.0,
                  category_name: "电子产品",
                  price_rank: 3,
                },
              ],
              explanation:
                "使用窗口函数ROW_NUMBER()对每个分类内的产品按价格排名，然后筛选前3名。",
            },
            按月统计订单数量和总金额: {
              success: true,
              sql: `SELECT DATE_FORMAT(created_at, '%Y-%m') as month,
       COUNT(*) as order_count,
       SUM(amount) as total_amount
FROM orders
GROUP BY DATE_FORMAT(created_at, '%Y-%m')
ORDER BY month;`,
              data: [
                { month: "2024-01", order_count: 156, total_amount: 45678.9 },
                { month: "2024-02", order_count: 189, total_amount: 52341.2 },
                { month: "2024-03", order_count: 203, total_amount: 58976.45 },
              ],
              explanation: "按月份分组统计订单数量和总金额，用于分析业务趋势。",
            },
          };

          if (responses[queryText]) {
            return responses[queryText];
          }

          return {
            success: true,
            sql: `-- 复杂查询: ${queryText}\nSELECT * FROM complex_query_result;`,
            data: [
              {
                result: "这是一个复杂查询的示例结果",
                query: queryText,
                type: "complex",
              },
            ],
            explanation: `这是对复杂查询 "${queryText}" 的解释说明，涉及多表关联、聚合计算等高级SQL特性。`,
          };
        }
      }

      const client = new ComplexRAGClient();

      // 切换标签页
      function showTab(tabName) {
        // 隐藏所有内容
        document.querySelectorAll(".query-content").forEach((content) => {
          content.classList.remove("active");
        });

        // 移除所有标签的active类
        document.querySelectorAll(".tab").forEach((tab) => {
          tab.classList.remove("active");
        });

        // 显示选中的内容
        document.getElementById(tabName).classList.add("active");

        // 激活选中的标签
        event.target.classList.add("active");
      }

      // 执行查询
      async function executeQuery(inputId, resultId) {
        const queryText = document.getElementById(inputId).value.trim();
        if (!queryText) {
          alert("请输入查询内容");
          return;
        }

        const resultDiv = document.getElementById(resultId);
        resultDiv.innerHTML = "<div>正在执行复杂查询...</div>";

        try {
          const result = await client.query(queryText);
          displayComplexResult(result, resultDiv);
        } catch (error) {
          resultDiv.innerHTML = `<div class="error">查询失败: ${error.message}</div>`;
        }
      }

      // 执行预设查询
      async function runPresetQuery(queryText, resultId) {
        const resultDiv = document.getElementById(resultId);
        resultDiv.innerHTML = "<div>正在执行复杂查询...</div>";

        try {
          const result = await client.query(queryText);
          displayComplexResult(result, resultDiv);
        } catch (error) {
          resultDiv.innerHTML = `<div class="error">查询失败: ${error.message}</div>`;
        }
      }

      // 显示复杂查询结果
      function displayComplexResult(result, container) {
        let html = "";

        if (result.success) {
          html += '<div class="success">复杂查询执行成功</div>';

          // 显示解释
          if (result.explanation) {
            html +=
              '<div class="explanation"><strong>查询解释:</strong> ' +
              result.explanation +
              "</div>";
          }

          // 显示 SQL
          if (result.sql) {
            html +=
              '<div class="sql-display"><strong>生成的 SQL:</strong><br><pre>' +
              result.sql +
              "</pre></div>";
          }

          // 显示数据
          if (result.data && result.data.length > 0) {
            html +=
              "<div><strong>查询结果 (" +
              result.data.length +
              " 条记录):</strong></div>";
            html += "<table>";

            // 表头
            const headers = Object.keys(result.data[0]);
            html += "<tr>";
            headers.forEach((header) => {
              html += "<th>" + header + "</th>";
            });
            html += "</tr>";

            // 数据行（限制显示前20行）
            const displayData = result.data.slice(0, 20);
            displayData.forEach((row) => {
              html += "<tr>";
              headers.forEach((header) => {
                let value = row[header];
                if (typeof value === "number" && value % 1 !== 0) {
                  value = value.toFixed(2);
                }
                html += "<td>" + (value || "") + "</td>";
              });
              html += "</tr>";
            });

            html += "</table>";

            if (result.data.length > 20) {
              html +=
                "<div><em>显示前20条记录，共" +
                result.data.length +
                "条</em></div>";
            }
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

## 性能优化建议

### 1. 索引优化

```javascript
// 对于频繁的关联查询，确保相关字段有索引
const result = await client.query("查询用户订单信息", {
  optimize: true,
  explain: true,
});
```

### 2. 查询限制

```javascript
// 复杂查询使用适当的限制
const result = await client.query("查询用户及其订单详情", {
  limit: 100,
  timeout: 30,
});
```

### 3. 分页处理

```javascript
// 大结果集使用分页
const result = await client.query("统计分析查询", {
  limit: 50,
  offset: 100,
});
```

## 常见问题

### Q: 复杂查询执行时间过长怎么办？

A: 使用查询优化选项，添加适当的限制条件，或者将复杂查询拆分为多个简单查询。

### Q: 如何处理大量数据的聚合查询？

A: 使用分页、时间范围限制，或者考虑使用预聚合的数据表。

### Q: 多表关联查询结果不准确？

A: 检查关联条件是否正确，考虑使用 LEFT JOIN 包含所有相关记录。

### Q: 窗口函数查询性能差？

A: 确保分区字段和排序字段有适当的索引，考虑限制数据范围。

## 下一步

- 学习 [时间序列查询](time-series.md)
- 了解 [统计分析查询](analytics.md)
- 查看 [性能优化指南](../advanced/performance.md)
- 阅读 [查询调优技巧](../advanced/query-tuning.md)
