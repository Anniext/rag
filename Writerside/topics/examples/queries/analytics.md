# 统计分析查询示例

本文档展示 RAG MySQL 查询系统中各种统计分析查询的使用方法，包括数据聚合、趋势分析、业务指标计算等高级分析场景。

## 基础统计分析

### 1. 计数统计

```javascript
// 统计用户总数
const result = await client.query("统计用户总数");

// 按状态统计用户数量
const result = await client.query("按状态统计用户数量");

// 统计活跃用户数量
const result = await client.query("统计状态为活跃的用户数量");

// 统计各分类的产品数量
const result = await client.query("统计各分类的产品数量");
```

**生成的 SQL 示例：**

```sql
SELECT COUNT(*) as total_users FROM users;

SELECT status, COUNT(*) as user_count
FROM users
GROUP BY status;

SELECT COUNT(*) as active_users
FROM users
WHERE status = 'active';

SELECT c.name, COUNT(p.id) as product_count
FROM categories c
LEFT JOIN products p ON c.id = p.category_id
GROUP BY c.id, c.name;
```

### 2. 求和统计

```javascript
// 统计订单总金额
const result = await client.query("统计所有订单的总金额");

// 按用户统计订单金额
const result = await client.query("统计各用户的订单总金额");

// 统计各分类产品的总价值
const result = await client.query("统计各分类产品的库存总价值");

// 统计今日销售总额
const result = await client.query("统计今天的销售总额");
```

**生成的 SQL 示例：**

```sql
SELECT SUM(amount) as total_amount FROM orders;

SELECT u.name, SUM(o.amount) as total_amount
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name;

SELECT c.name, SUM(p.price * p.stock_quantity) as total_value
FROM categories c
JOIN products p ON c.id = p.category_id
GROUP BY c.id, c.name;

SELECT SUM(amount) as daily_sales
FROM orders
WHERE DATE(created_at) = CURDATE();
```

### 3. 平均值统计

```javascript
// 计算平均订单金额
const result = await client.query("计算平均订单金额");

// 计算各分类产品的平均价格
const result = await client.query("计算各分类产品的平均价格");

// 计算用户平均消费金额
const result = await client.query("计算用户平均消费金额");

// 计算月平均销售额
const result = await client.query("计算月平均销售额");
```

**生成的 SQL 示例：**

```sql
SELECT AVG(amount) as avg_order_amount FROM orders;

SELECT c.name, AVG(p.price) as avg_price
FROM categories c
JOIN products p ON c.id = p.category_id
GROUP BY c.id, c.name;

SELECT AVG(user_total) as avg_user_spending
FROM (
    SELECT u.id, SUM(o.amount) as user_total
    FROM users u
    JOIN orders o ON u.id = o.user_id
    GROUP BY u.id
) user_totals;

SELECT AVG(monthly_sales) as avg_monthly_sales
FROM (
    SELECT DATE_FORMAT(created_at, '%Y-%m') as month,
           SUM(amount) as monthly_sales
    FROM orders
    GROUP BY DATE_FORMAT(created_at, '%Y-%m')
) monthly_data;
```

## 时间序列分析

### 1. 按时间分组统计

```javascript
// 按月统计订单数量和金额
const result = await client.query("按月统计订单数量和总金额");

// 按周统计用户注册数量
const result = await client.query("按周统计用户注册数量");

// 按日统计销售情况
const result = await client.query("按日统计最近30天的销售情况");

// 按小时统计订单分布
const result = await client.query("按小时统计今天的订单分布");
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

SELECT DATE(created_at) as date,
       COUNT(*) as order_count,
       SUM(amount) as daily_sales
FROM orders
WHERE created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)
GROUP BY DATE(created_at)
ORDER BY date;

SELECT HOUR(created_at) as hour,
       COUNT(*) as order_count
FROM orders
WHERE DATE(created_at) = CURDATE()
GROUP BY HOUR(created_at)
ORDER BY hour;
```

### 2. 趋势分析

```javascript
// 分析用户注册趋势
const result = await client.query("分析最近12个月的用户注册趋势");

// 分析销售增长趋势
const result = await client.query("分析销售额的月度增长趋势");

// 分析产品销量趋势
const result = await client.query("分析各产品的销量趋势");

// 分析季度业绩对比
const result = await client.query("分析各季度的业绩对比");
```

**生成的 SQL 示例：**

```sql
SELECT DATE_FORMAT(created_at, '%Y-%m') as month,
       COUNT(*) as new_users,
       SUM(COUNT(*)) OVER (ORDER BY DATE_FORMAT(created_at, '%Y-%m')) as cumulative_users
FROM users
WHERE created_at >= DATE_SUB(NOW(), INTERVAL 12 MONTH)
GROUP BY DATE_FORMAT(created_at, '%Y-%m')
ORDER BY month;

SELECT month,
       monthly_sales,
       LAG(monthly_sales) OVER (ORDER BY month) as prev_month_sales,
       ROUND((monthly_sales - LAG(monthly_sales) OVER (ORDER BY month)) * 100.0 /
             LAG(monthly_sales) OVER (ORDER BY month), 2) as growth_rate
FROM (
    SELECT DATE_FORMAT(created_at, '%Y-%m') as month,
           SUM(amount) as monthly_sales
    FROM orders
    GROUP BY DATE_FORMAT(created_at, '%Y-%m')
) monthly_stats
ORDER BY month;

SELECT p.name,
       DATE_FORMAT(o.created_at, '%Y-%m') as month,
       SUM(oi.quantity) as monthly_quantity
FROM products p
JOIN order_items oi ON p.id = oi.product_id
JOIN orders o ON oi.order_id = o.id
GROUP BY p.id, p.name, DATE_FORMAT(o.created_at, '%Y-%m')
ORDER BY p.name, month;

SELECT QUARTER(created_at) as quarter,
       YEAR(created_at) as year,
       COUNT(*) as order_count,
       SUM(amount) as quarterly_sales
FROM orders
GROUP BY YEAR(created_at), QUARTER(created_at)
ORDER BY year, quarter;
```

## 业务指标分析

### 1. 用户行为分析

```javascript
// 分析用户购买频次
const result = await client.query("分析用户的购买频次分布");

// 分析用户生命周期价值
const result = await client.query("分析用户的生命周期价值");

// 分析用户复购率
const result = await client.query("分析用户的复购率");

// 分析用户流失情况
const result = await client.query("分析用户流失情况");
```

**生成的 SQL 示例：**

```sql
SELECT purchase_frequency,
       COUNT(*) as user_count
FROM (
    SELECT u.id,
           COUNT(o.id) as purchase_frequency
    FROM users u
    LEFT JOIN orders o ON u.id = o.user_id
    GROUP BY u.id
) user_purchases
GROUP BY purchase_frequency
ORDER BY purchase_frequency;

SELECT u.name,
       COUNT(o.id) as total_orders,
       SUM(o.amount) as lifetime_value,
       AVG(o.amount) as avg_order_value,
       DATEDIFF(MAX(o.created_at), MIN(o.created_at)) as customer_lifespan_days
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
HAVING COUNT(o.id) > 1
ORDER BY lifetime_value DESC;

SELECT
    COUNT(DISTINCT first_purchase.user_id) as total_customers,
    COUNT(DISTINCT repeat_purchase.user_id) as repeat_customers,
    ROUND(COUNT(DISTINCT repeat_purchase.user_id) * 100.0 /
          COUNT(DISTINCT first_purchase.user_id), 2) as repeat_rate
FROM (
    SELECT user_id, MIN(created_at) as first_order_date
    FROM orders
    GROUP BY user_id
) first_purchase
LEFT JOIN orders repeat_purchase ON first_purchase.user_id = repeat_purchase.user_id
    AND repeat_purchase.created_at > first_purchase.first_order_date;

SELECT
    CASE
        WHEN DATEDIFF(NOW(), last_order_date) <= 30 THEN '活跃用户'
        WHEN DATEDIFF(NOW(), last_order_date) <= 90 THEN '风险用户'
        ELSE '流失用户'
    END as user_status,
    COUNT(*) as user_count
FROM (
    SELECT u.id,
           MAX(o.created_at) as last_order_date
    FROM users u
    LEFT JOIN orders o ON u.id = o.user_id
    GROUP BY u.id
) user_last_order
GROUP BY
    CASE
        WHEN DATEDIFF(NOW(), last_order_date) <= 30 THEN '活跃用户'
        WHEN DATEDIFF(NOW(), last_order_date) <= 90 THEN '风险用户'
        ELSE '流失用户'
    END;
```

### 2. 产品销售分析

```javascript
// 分析产品销售排行
const result = await client.query("分析产品销售排行榜");

// 分析产品利润贡献
const result = await client.query("分析各产品的利润贡献");

// 分析产品库存周转率
const result = await client.query("分析产品库存周转率");

// 分析产品价格敏感度
const result = await client.query("分析产品价格对销量的影响");
```

**生成的 SQL 示例：**

```sql
SELECT p.name,
       SUM(oi.quantity) as total_sold,
       SUM(oi.total_price) as total_revenue,
       COUNT(DISTINCT oi.order_id) as order_count,
       RANK() OVER (ORDER BY SUM(oi.quantity) DESC) as sales_rank
FROM products p
JOIN order_items oi ON p.id = oi.product_id
GROUP BY p.id, p.name
ORDER BY total_sold DESC;

SELECT p.name,
       p.price,
       SUM(oi.quantity) as units_sold,
       SUM(oi.total_price) as revenue,
       SUM(oi.total_price) - (SUM(oi.quantity) * p.cost) as profit,
       ROUND((SUM(oi.total_price) - (SUM(oi.quantity) * p.cost)) * 100.0 /
             SUM(oi.total_price), 2) as profit_margin
FROM products p
JOIN order_items oi ON p.id = oi.product_id
GROUP BY p.id, p.name, p.price, p.cost
ORDER BY profit DESC;

SELECT p.name,
       p.stock_quantity as current_stock,
       SUM(oi.quantity) as sold_quantity,
       ROUND(SUM(oi.quantity) / p.stock_quantity, 2) as turnover_rate,
       CASE
           WHEN SUM(oi.quantity) / p.stock_quantity > 2 THEN '高周转'
           WHEN SUM(oi.quantity) / p.stock_quantity > 1 THEN '中周转'
           ELSE '低周转'
       END as turnover_category
FROM products p
JOIN order_items oi ON p.id = oi.product_id
WHERE p.stock_quantity > 0
GROUP BY p.id, p.name, p.stock_quantity
ORDER BY turnover_rate DESC;

SELECT price_range,
       AVG(units_sold) as avg_units_sold,
       COUNT(*) as product_count
FROM (
    SELECT p.name,
           CASE
               WHEN p.price < 50 THEN '低价位(0-50)'
               WHEN p.price < 200 THEN '中价位(50-200)'
               WHEN p.price < 500 THEN '高价位(200-500)'
               ELSE '奢侈品(500+)'
           END as price_range,
           SUM(oi.quantity) as units_sold
    FROM products p
    JOIN order_items oi ON p.id = oi.product_id
    GROUP BY p.id, p.name, p.price
) product_sales
GROUP BY price_range
ORDER BY avg_units_sold DESC;
```

### 3. 财务分析

```javascript
// 分析收入构成
const result = await client.query("分析收入构成按分类");

// 分析成本结构
const result = await client.query("分析成本结构");

// 分析利润率趋势
const result = await client.query("分析月度利润率趋势");

// 分析现金流情况
const result = await client.query("分析现金流情况");
```

**生成的 SQL 示例：**

```sql
SELECT c.name as category,
       SUM(oi.total_price) as category_revenue,
       ROUND(SUM(oi.total_price) * 100.0 /
             (SELECT SUM(total_price) FROM order_items), 2) as revenue_percentage
FROM categories c
JOIN products p ON c.id = p.category_id
JOIN order_items oi ON p.id = oi.product_id
GROUP BY c.id, c.name
ORDER BY category_revenue DESC;

SELECT
    SUM(oi.total_price) as total_revenue,
    SUM(oi.quantity * p.cost) as total_cost,
    SUM(oi.total_price) - SUM(oi.quantity * p.cost) as gross_profit,
    ROUND((SUM(oi.total_price) - SUM(oi.quantity * p.cost)) * 100.0 /
          SUM(oi.total_price), 2) as gross_margin
FROM order_items oi
JOIN products p ON oi.product_id = p.id;

SELECT DATE_FORMAT(o.created_at, '%Y-%m') as month,
       SUM(oi.total_price) as monthly_revenue,
       SUM(oi.quantity * p.cost) as monthly_cost,
       SUM(oi.total_price) - SUM(oi.quantity * p.cost) as monthly_profit,
       ROUND((SUM(oi.total_price) - SUM(oi.quantity * p.cost)) * 100.0 /
             SUM(oi.total_price), 2) as profit_margin
FROM orders o
JOIN order_items oi ON o.id = oi.order_id
JOIN products p ON oi.product_id = p.id
GROUP BY DATE_FORMAT(o.created_at, '%Y-%m')
ORDER BY month;

SELECT DATE_FORMAT(created_at, '%Y-%m') as month,
       SUM(CASE WHEN status = 'paid' THEN amount ELSE 0 END) as cash_inflow,
       SUM(CASE WHEN status = 'refunded' THEN amount ELSE 0 END) as cash_outflow,
       SUM(CASE WHEN status = 'paid' THEN amount ELSE 0 END) -
       SUM(CASE WHEN status = 'refunded' THEN amount ELSE 0 END) as net_cash_flow
FROM orders
GROUP BY DATE_FORMAT(created_at, '%Y-%m')
ORDER BY month;
```

## 高级分析查询

### 1. 同期对比分析

```javascript
// 同比增长分析
const result = await client.query("分析销售额的同比增长情况");

// 环比增长分析
const result = await client.query("分析订单数量的环比增长情况");

// 季度对比分析
const result = await client.query("分析各季度业绩对比");

// 工作日vs周末分析
const result = await client.query("分析工作日和周末的销售对比");
```

**生成的 SQL 示例：**

```sql
SELECT current_year.month,
       current_year.sales as current_year_sales,
       last_year.sales as last_year_sales,
       ROUND((current_year.sales - last_year.sales) * 100.0 /
             last_year.sales, 2) as yoy_growth_rate
FROM (
    SELECT DATE_FORMAT(created_at, '%m') as month,
           SUM(amount) as sales
    FROM orders
    WHERE YEAR(created_at) = YEAR(NOW())
    GROUP BY DATE_FORMAT(created_at, '%m')
) current_year
LEFT JOIN (
    SELECT DATE_FORMAT(created_at, '%m') as month,
           SUM(amount) as sales
    FROM orders
    WHERE YEAR(created_at) = YEAR(NOW()) - 1
    GROUP BY DATE_FORMAT(created_at, '%m')
) last_year ON current_year.month = last_year.month
ORDER BY current_year.month;

SELECT month,
       order_count,
       LAG(order_count) OVER (ORDER BY month) as prev_month_count,
       ROUND((order_count - LAG(order_count) OVER (ORDER BY month)) * 100.0 /
             LAG(order_count) OVER (ORDER BY month), 2) as mom_growth_rate
FROM (
    SELECT DATE_FORMAT(created_at, '%Y-%m') as month,
           COUNT(*) as order_count
    FROM orders
    GROUP BY DATE_FORMAT(created_at, '%Y-%m')
) monthly_orders
ORDER BY month;

SELECT YEAR(created_at) as year,
       QUARTER(created_at) as quarter,
       COUNT(*) as order_count,
       SUM(amount) as quarterly_sales,
       RANK() OVER (PARTITION BY YEAR(created_at) ORDER BY SUM(amount) DESC) as quarter_rank
FROM orders
GROUP BY YEAR(created_at), QUARTER(created_at)
ORDER BY year, quarter;

SELECT
    CASE
        WHEN DAYOFWEEK(created_at) IN (1, 7) THEN '周末'
        ELSE '工作日'
    END as day_type,
    COUNT(*) as order_count,
    SUM(amount) as total_sales,
    AVG(amount) as avg_order_value
FROM orders
GROUP BY
    CASE
        WHEN DAYOFWEEK(created_at) IN (1, 7) THEN '周末'
        ELSE '工作日'
    END;
```

### 2. 客户细分分析

```javascript
// RFM 客户分析
const result = await client.query("进行RFM客户价值分析");

// 客户生命周期分析
const result = await client.query("分析客户生命周期阶段");

// 高价值客户识别
const result = await client.query("识别高价值客户");

// 客户流失预警
const result = await client.query("客户流失风险预警分析");
```

**生成的 SQL 示例：**

```sql
SELECT user_id,
       name,
       recency_score,
       frequency_score,
       monetary_score,
       CASE
           WHEN recency_score >= 4 AND frequency_score >= 4 AND monetary_score >= 4 THEN '冠军客户'
           WHEN recency_score >= 3 AND frequency_score >= 3 AND monetary_score >= 3 THEN '忠诚客户'
           WHEN recency_score >= 3 AND frequency_score <= 2 THEN '潜在客户'
           WHEN recency_score <= 2 AND frequency_score >= 3 THEN '风险客户'
           ELSE '一般客户'
       END as customer_segment
FROM (
    SELECT u.id as user_id,
           u.name,
           NTILE(5) OVER (ORDER BY DATEDIFF(NOW(), MAX(o.created_at))) as recency_score,
           NTILE(5) OVER (ORDER BY COUNT(o.id)) as frequency_score,
           NTILE(5) OVER (ORDER BY SUM(o.amount)) as monetary_score
    FROM users u
    LEFT JOIN orders o ON u.id = o.user_id
    GROUP BY u.id, u.name
) rfm_scores;

SELECT customer_lifecycle,
       COUNT(*) as customer_count,
       AVG(total_spent) as avg_spending,
       AVG(order_count) as avg_orders
FROM (
    SELECT u.id,
           u.name,
           COUNT(o.id) as order_count,
           SUM(o.amount) as total_spent,
           DATEDIFF(NOW(), MAX(o.created_at)) as days_since_last_order,
           CASE
               WHEN COUNT(o.id) = 0 THEN '潜在客户'
               WHEN COUNT(o.id) = 1 AND DATEDIFF(NOW(), MAX(o.created_at)) <= 30 THEN '新客户'
               WHEN COUNT(o.id) > 1 AND DATEDIFF(NOW(), MAX(o.created_at)) <= 30 THEN '活跃客户'
               WHEN DATEDIFF(NOW(), MAX(o.created_at)) BETWEEN 31 AND 90 THEN '风险客户'
               ELSE '流失客户'
           END as customer_lifecycle
    FROM users u
    LEFT JOIN orders o ON u.id = o.user_id
    GROUP BY u.id, u.name
) customer_analysis
GROUP BY customer_lifecycle;

SELECT u.name,
       COUNT(o.id) as total_orders,
       SUM(o.amount) as lifetime_value,
       AVG(o.amount) as avg_order_value,
       MAX(o.created_at) as last_order_date,
       DATEDIFF(NOW(), MAX(o.created_at)) as days_since_last_order
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
HAVING SUM(o.amount) > (
    SELECT AVG(user_total) * 2
    FROM (
        SELECT SUM(amount) as user_total
        FROM orders
        GROUP BY user_id
    ) user_totals
)
ORDER BY lifetime_value DESC;

SELECT u.name,
       COUNT(o.id) as order_count,
       SUM(o.amount) as total_spent,
       MAX(o.created_at) as last_order_date,
       DATEDIFF(NOW(), MAX(o.created_at)) as days_inactive,
       CASE
           WHEN DATEDIFF(NOW(), MAX(o.created_at)) > 90 AND SUM(o.amount) > 1000 THEN '高价值流失风险'
           WHEN DATEDIFF(NOW(), MAX(o.created_at)) > 60 AND COUNT(o.id) > 5 THEN '活跃客户流失风险'
           WHEN DATEDIFF(NOW(), MAX(o.created_at)) > 30 AND COUNT(o.id) <= 2 THEN '新客户流失风险'
           ELSE '正常'
       END as churn_risk
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
HAVING COUNT(o.id) > 0 AND DATEDIFF(NOW(), MAX(o.created_at)) > 30
ORDER BY days_inactive DESC;
```

## 完整分析示例应用

```html
<!DOCTYPE html>
<html>
  <head>
    <title>统计分析查询示例</title>
    <style>
      .container {
        max-width: 1400px;
        margin: 0 auto;
        padding: 20px;
      }
      .analysis-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
        gap: 20px;
        margin: 20px 0;
      }
      .analysis-card {
        border: 1px solid #ddd;
        border-radius: 8px;
        padding: 20px;
        background: #f9f9f9;
      }
      .analysis-card h3 {
        margin-top: 0;
        color: #333;
      }
      .query-button {
        padding: 10px 15px;
        margin: 5px;
        background: #007bff;
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
      }
      .query-button:hover {
        background: #0056b3;
      }
      .results {
        margin: 15px 0;
        max-height: 300px;
        overflow-y: auto;
      }
      .chart-container {
        width: 100%;
        height: 200px;
        background: #fff;
        border: 1px solid #ddd;
        margin: 10px 0;
        display: flex;
        align-items: center;
        justify-content: center;
      }
      table {
        width: 100%;
        border-collapse: collapse;
        margin: 10px 0;
        font-size: 12px;
      }
      th,
      td {
        border: 1px solid #ddd;
        padding: 6px;
        text-align: left;
      }
      th {
        background-color: #f2f2f2;
      }
      .metric {
        display: inline-block;
        margin: 10px;
        padding: 15px;
        background: #e7f3ff;
        border-radius: 5px;
        text-align: center;
      }
      .metric-value {
        font-size: 24px;
        font-weight: bold;
        color: #007bff;
      }
      .metric-label {
        font-size: 12px;
        color: #666;
      }
    </style>
  </head>
  <body>
    <div class="container">
      <h1>RAG MySQL 查询系统 - 统计分析示例</h1>

      <div class="analysis-grid">
        <!-- 基础统计 -->
        <div class="analysis-card">
          <h3>基础统计分析</h3>
          <button
            class="query-button"
            onclick="runAnalysis('统计用户总数', 'basic-stats')"
          >
            用户总数
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('按状态统计用户数量', 'basic-stats')"
          >
            用户状态分布
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('统计订单总金额', 'basic-stats')"
          >
            订单总金额
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('计算平均订单金额', 'basic-stats')"
          >
            平均订单金额
          </button>
          <div id="basic-stats" class="results"></div>
        </div>

        <!-- 时间趋势 -->
        <div class="analysis-card">
          <h3>时间趋势分析</h3>
          <button
            class="query-button"
            onclick="runAnalysis('按月统计订单数量和总金额', 'time-trend')"
          >
            月度趋势
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('按周统计用户注册数量', 'time-trend')"
          >
            注册趋势
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('分析销售额的月度增长趋势', 'time-trend')"
          >
            增长趋势
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('按小时统计今天的订单分布', 'time-trend')"
          >
            小时分布
          </button>
          <div id="time-trend" class="results"></div>
        </div>

        <!-- 用户分析 -->
        <div class="analysis-card">
          <h3>用户行为分析</h3>
          <button
            class="query-button"
            onclick="runAnalysis('分析用户的购买频次分布', 'user-behavior')"
          >
            购买频次
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('分析用户的生命周期价值', 'user-behavior')"
          >
            生命周期价值
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('分析用户的复购率', 'user-behavior')"
          >
            复购率
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('识别高价值客户', 'user-behavior')"
          >
            高价值客户
          </button>
          <div id="user-behavior" class="results"></div>
        </div>

        <!-- 产品分析 -->
        <div class="analysis-card">
          <h3>产品销售分析</h3>
          <button
            class="query-button"
            onclick="runAnalysis('分析产品销售排行榜', 'product-analysis')"
          >
            销售排行
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('分析各产品的利润贡献', 'product-analysis')"
          >
            利润贡献
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('分析产品库存周转率', 'product-analysis')"
          >
            库存周转
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('统计各分类的产品数量', 'product-analysis')"
          >
            分类统计
          </button>
          <div id="product-analysis" class="results"></div>
        </div>

        <!-- 财务分析 -->
        <div class="analysis-card">
          <h3>财务分析</h3>
          <button
            class="query-button"
            onclick="runAnalysis('分析收入构成按分类', 'financial-analysis')"
          >
            收入构成
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('分析月度利润率趋势', 'financial-analysis')"
          >
            利润率趋势
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('分析现金流情况', 'financial-analysis')"
          >
            现金流
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('分析成本结构', 'financial-analysis')"
          >
            成本结构
          </button>
          <div id="financial-analysis" class="results"></div>
        </div>

        <!-- 对比分析 -->
        <div class="analysis-card">
          <h3>对比分析</h3>
          <button
            class="query-button"
            onclick="runAnalysis('分析销售额的同比增长情况', 'comparison-analysis')"
          >
            同比增长
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('分析各季度业绩对比', 'comparison-analysis')"
          >
            季度对比
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('分析工作日和周末的销售对比', 'comparison-analysis')"
          >
            工作日对比
          </button>
          <button
            class="query-button"
            onclick="runAnalysis('进行RFM客户价值分析', 'comparison-analysis')"
          >
            RFM分析
          </button>
          <div id="comparison-analysis" class="results"></div>
        </div>
      </div>

      <!-- 综合仪表板 -->
      <div class="analysis-card" style="margin-top: 20px;">
        <h3>综合业务仪表板</h3>
        <button class="query-button" onclick="loadDashboard()">
          加载仪表板
        </button>
        <div id="dashboard" class="results"></div>
      </div>
    </div>

    <script>
      // 模拟分析查询客户端
      class AnalyticsRAGClient {
        async query(queryText) {
          await new Promise((resolve) => setTimeout(resolve, 1000));
          return this.generateAnalyticsResponse(queryText);
        }

        generateAnalyticsResponse(queryText) {
          const responses = {
            统计用户总数: {
              success: true,
              sql: "SELECT COUNT(*) as total_users FROM users;",
              data: [{ total_users: 1256 }],
              explanation: "统计系统中的用户总数。",
              metrics: [{ label: "总用户数", value: "1,256", trend: "+12%" }],
            },
            按状态统计用户数量: {
              success: true,
              sql: "SELECT status, COUNT(*) as user_count FROM users GROUP BY status;",
              data: [
                { status: "active", user_count: 1089 },
                { status: "inactive", user_count: 142 },
                { status: "suspended", user_count: 25 },
              ],
              explanation: "按用户状态分组统计用户数量分布。",
            },
            按月统计订单数量和总金额: {
              success: true,
              sql: `SELECT DATE_FORMAT(created_at, '%Y-%m') as month, COUNT(*) as order_count, SUM(amount) as total_amount FROM orders GROUP BY DATE_FORMAT(created_at, '%Y-%m') ORDER BY month;`,
              data: [
                { month: "2024-01", order_count: 234, total_amount: 45678.9 },
                { month: "2024-02", order_count: 267, total_amount: 52341.2 },
                { month: "2024-03", order_count: 298, total_amount: 58976.45 },
                { month: "2024-04", order_count: 312, total_amount: 61234.78 },
              ],
              explanation: "按月份统计订单数量和总金额，展示业务增长趋势。",
            },
            分析产品销售排行榜: {
              success: true,
              sql: `SELECT p.name, SUM(oi.quantity) as total_sold, SUM(oi.total_price) as total_revenue FROM products p JOIN order_items oi ON p.id = oi.product_id GROUP BY p.id, p.name ORDER BY total_sold DESC;`,
              data: [
                { name: "智能手机", total_sold: 456, total_revenue: 234567.89 },
                { name: "蓝牙耳机", total_sold: 389, total_revenue: 89234.56 },
                { name: "平板电脑", total_sold: 234, total_revenue: 156789.23 },
                { name: "智能手表", total_sold: 198, total_revenue: 98765.43 },
              ],
              explanation: "按销售数量排序的产品销售排行榜。",
            },
            分析用户的生命周期价值: {
              success: true,
              sql: `SELECT u.name, COUNT(o.id) as total_orders, SUM(o.amount) as lifetime_value, AVG(o.amount) as avg_order_value FROM users u JOIN orders o ON u.id = o.user_id GROUP BY u.id, u.name ORDER BY lifetime_value DESC;`,
              data: [
                {
                  name: "张三",
                  total_orders: 12,
                  lifetime_value: 5678.9,
                  avg_order_value: 473.24,
                },
                {
                  name: "李四",
                  total_orders: 8,
                  lifetime_value: 4321.56,
                  avg_order_value: 540.2,
                },
                {
                  name: "王五",
                  total_orders: 15,
                  lifetime_value: 3987.65,
                  avg_order_value: 265.84,
                },
              ],
              explanation:
                "分析用户的生命周期价值，包括订单数量、总消费和平均订单价值。",
            },
          };

          return (
            responses[queryText] || {
              success: true,
              sql: `-- 分析查询: ${queryText}\nSELECT * FROM analytics_result;`,
              data: [
                {
                  result: "分析结果示例",
                  metric: Math.floor(Math.random() * 1000),
                },
              ],
              explanation: `这是对分析查询 "${queryText}" 的结果展示。`,
            }
          );
        }
      }

      const client = new AnalyticsRAGClient();

      // 执行分析查询
      async function runAnalysis(queryText, containerId) {
        const container = document.getElementById(containerId);
        container.innerHTML = "<div>正在执行分析...</div>";

        try {
          const result = await client.query(queryText);
          displayAnalysisResult(result, container);
        } catch (error) {
          container.innerHTML = `<div style="color: red;">分析失败: ${error.message}</div>`;
        }
      }

      // 显示分析结果
      function displayAnalysisResult(result, container) {
        let html = "";

        // 显示指标卡片
        if (result.metrics) {
          html += '<div style="margin: 10px 0;">';
          result.metrics.forEach((metric) => {
            html += `<div class="metric">
                        <div class="metric-value">${metric.value}</div>
                        <div class="metric-label">${metric.label}</div>
                        ${
                          metric.trend
                            ? `<div style="color: green; font-size: 12px;">${metric.trend}</div>`
                            : ""
                        }
                    </div>`;
          });
          html += "</div>";
        }

        // 显示解释
        if (result.explanation) {
          html += `<div style="background: #e7f3ff; padding: 10px; margin: 10px 0; border-radius: 5px;">
                    <strong>分析说明:</strong> ${result.explanation}
                </div>`;
        }

        // 显示数据表格
        if (result.data && result.data.length > 0) {
          html += "<table>";

          // 表头
          const headers = Object.keys(result.data[0]);
          html += "<tr>";
          headers.forEach((header) => {
            html += `<th>${header}</th>`;
          });
          html += "</tr>";

          // 数据行
          result.data.forEach((row) => {
            html += "<tr>";
            headers.forEach((header) => {
              let value = row[header];
              if (typeof value === "number" && value % 1 !== 0) {
                value = value.toLocaleString("zh-CN", {
                  minimumFractionDigits: 2,
                  maximumFractionDigits: 2,
                });
              } else if (typeof value === "number") {
                value = value.toLocaleString("zh-CN");
              }
              html += `<td>${value || ""}</td>`;
            });
            html += "</tr>";
          });

          html += "</table>";
        }

        // 显示 SQL
        if (result.sql) {
          html += `<details style="margin: 10px 0;">
                    <summary>查看生成的 SQL</summary>
                    <pre style="background: #f5f5f5; padding: 10px; border-radius: 5px; overflow-x: auto;">${result.sql}</pre>
                </details>`;
        }

        container.innerHTML = html;
      }

      // 加载综合仪表板
      async function loadDashboard() {
        const container = document.getElementById("dashboard");
        container.innerHTML = "<div>正在加载仪表板数据...</div>";

        try {
          // 并行执行多个分析查询
          const [userStats, orderStats, productStats, revenueStats] =
            await Promise.all([
              client.query("统计用户总数"),
              client.query("统计订单总金额"),
              client.query("分析产品销售排行榜"),
              client.query("按月统计订单数量和总金额"),
            ]);

          let html =
            '<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 15px;">';

          // 用户统计卡片
          html +=
            '<div style="background: white; padding: 15px; border-radius: 8px; border: 1px solid #ddd;">';
          html += "<h4>用户统计</h4>";
          html += `<div class="metric-value" style="color: #28a745;">${userStats.data[0].total_users.toLocaleString()}</div>`;
          html += '<div class="metric-label">总用户数</div>';
          html += "</div>";

          // 订单统计卡片
          html +=
            '<div style="background: white; padding: 15px; border-radius: 8px; border: 1px solid #ddd;">';
          html += "<h4>销售统计</h4>";
          const totalRevenue = revenueStats.data.reduce(
            (sum, item) => sum + item.total_amount,
            0
          );
          html += `<div class="metric-value" style="color: #007bff;">¥${totalRevenue.toLocaleString(
            "zh-CN",
            { minimumFractionDigits: 2 }
          )}</div>`;
          html += '<div class="metric-label">总销售额</div>';
          html += "</div>";

          // 热销产品卡片
          html +=
            '<div style="background: white; padding: 15px; border-radius: 8px; border: 1px solid #ddd;">';
          html += "<h4>热销产品</h4>";
          html += `<div class="metric-value" style="color: #ffc107;">${productStats.data[0].name}</div>`;
          html += `<div class="metric-label">销量: ${productStats.data[0].total_sold} 件</div>`;
          html += "</div>";

          // 月度趋势卡片
          html +=
            '<div style="background: white; padding: 15px; border-radius: 8px; border: 1px solid #ddd;">';
          html += "<h4>增长趋势</h4>";
          const lastMonth = revenueStats.data[revenueStats.data.length - 1];
          const prevMonth = revenueStats.data[revenueStats.data.length - 2];
          const growth = (
            ((lastMonth.total_amount - prevMonth.total_amount) /
              prevMonth.total_amount) *
            100
          ).toFixed(1);
          html += `<div class="metric-value" style="color: ${
            growth > 0 ? "#28a745" : "#dc3545"
          };">+${growth}%</div>`;
          html += '<div class="metric-label">月度增长率</div>';
          html += "</div>";

          html += "</div>";

          // 添加趋势图表占位符
          html += '<div style="margin-top: 20px;">';
          html += "<h4>月度销售趋势</h4>";
          html +=
            '<div class="chart-container">📈 趋势图表 (集成 Chart.js 或其他图表库)</div>';
          html += "</div>";

          container.innerHTML = html;
        } catch (error) {
          container.innerHTML = `<div style="color: red;">仪表板加载失败: ${error.message}</div>`;
        }
      }
    </script>
  </body>
</html>
```

## 性能优化建议

### 1. 查询优化

- 使用适当的索引加速聚合查询
- 对于大数据量的分析，考虑使用分页或时间范围限制
- 复杂的统计查询可以考虑使用物化视图

### 2. 缓存策略

- 对于不经常变化的统计数据，启用查询缓存
- 使用 Redis 缓存热门的分析结果
- 实现增量更新机制

### 3. 数据预处理

- 对于复杂的分析查询，可以考虑预计算结果
- 使用定时任务更新统计数据
- 建立数据仓库或 OLAP 系统

## 常见问题

### Q: 如何处理大数据量的统计查询？

A: 使用时间范围限制、分页查询，或者考虑使用数据仓库解决方案。

### Q: 统计结果不准确怎么办？

A: 检查数据完整性，确认查询逻辑，考虑数据清洗和标准化。

### Q: 如何提高复杂分析查询的性能？

A: 优化数据库索引，使用查询缓存，考虑读写分离架构。

### Q: 如何实现实时统计分析？

A: 使用流处理技术，建立实时数据管道，或者使用内存数据库。

## 下一步

- 学习 [时间序列查询](time-series.md)
- 了解 [性能优化指南](../advanced/performance.md)
- 查看 [数据可视化集成](../advanced/visualization.md)
- 阅读 [商业智能应用](../advanced/business-intelligence.md)
