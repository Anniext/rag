package schema

import (
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
)

func createTestSchemaForSearch() *core.SchemaInfo {
	return &core.SchemaInfo{
		Database:  "test_db",
		Version:   "8.0.25",
		UpdatedAt: time.Now(),
		Tables: []*core.TableInfo{
			{
				Name:    "users",
				Comment: "用户表",
				Engine:  "InnoDB",
				Columns: []*core.Column{
					{
						Name:         "id",
						Type:         "bigint",
						IsPrimaryKey: true,
						Comment:      "用户ID",
						Position:     1,
					},
					{
						Name:     "username",
						Type:     "varchar(50)",
						Comment:  "用户名",
						Position: 2,
					},
					{
						Name:     "email",
						Type:     "varchar(100)",
						Comment:  "邮箱地址",
						Position: 3,
					},
					{
						Name:     "created_at",
						Type:     "timestamp",
						Comment:  "创建时间",
						Position: 4,
					},
				},
				Indexes: []*core.Index{
					{
						Name:     "PRIMARY",
						Type:     "PRIMARY",
						Columns:  []string{"id"},
						IsUnique: true,
					},
					{
						Name:     "idx_username",
						Type:     "INDEX",
						Columns:  []string{"username"},
						IsUnique: true,
					},
					{
						Name:     "idx_email",
						Type:     "INDEX",
						Columns:  []string{"email"},
						IsUnique: true,
					},
				},
				ForeignKeys: []*core.ForeignKey{},
				RowCount:    1000,
				UpdatedAt:   time.Now().Add(-time.Hour),
			},
			{
				Name:    "posts",
				Comment: "文章表",
				Engine:  "InnoDB",
				Columns: []*core.Column{
					{
						Name:         "id",
						Type:         "bigint",
						IsPrimaryKey: true,
						Comment:      "文章ID",
						Position:     1,
					},
					{
						Name:     "user_id",
						Type:     "bigint",
						Comment:  "作者ID",
						Position: 2,
					},
					{
						Name:     "title",
						Type:     "varchar(200)",
						Comment:  "文章标题",
						Position: 3,
					},
					{
						Name:     "content",
						Type:     "text",
						Comment:  "文章内容",
						Position: 4,
					},
					{
						Name:     "status",
						Type:     "varchar(20)",
						Comment:  "发布状态",
						Position: 5,
					},
					{
						Name:     "created_at",
						Type:     "timestamp",
						Comment:  "创建时间",
						Position: 6,
					},
				},
				Indexes: []*core.Index{
					{
						Name:     "PRIMARY",
						Type:     "PRIMARY",
						Columns:  []string{"id"},
						IsUnique: true,
					},
					{
						Name:     "idx_user_id",
						Type:     "INDEX",
						Columns:  []string{"user_id"},
						IsUnique: false,
					},
					{
						Name:     "idx_status",
						Type:     "INDEX",
						Columns:  []string{"status"},
						IsUnique: false,
					},
				},
				ForeignKeys: []*core.ForeignKey{
					{
						Name:             "fk_posts_user_id",
						Column:           "user_id",
						ReferencedTable:  "users",
						ReferencedColumn: "id",
					},
				},
				RowCount:  5000,
				UpdatedAt: time.Now().Add(-30 * time.Minute),
			},
			{
				Name:    "comments",
				Comment: "评论表",
				Engine:  "InnoDB",
				Columns: []*core.Column{
					{
						Name:         "id",
						Type:         "bigint",
						IsPrimaryKey: true,
						Comment:      "评论ID",
						Position:     1,
					},
					{
						Name:     "post_id",
						Type:     "bigint",
						Comment:  "文章ID",
						Position: 2,
					},
					{
						Name:     "user_id",
						Type:     "bigint",
						Comment:  "评论者ID",
						Position: 3,
					},
					{
						Name:     "content",
						Type:     "text",
						Comment:  "评论内容",
						Position: 4,
					},
				},
				Indexes: []*core.Index{
					{
						Name:     "PRIMARY",
						Type:     "PRIMARY",
						Columns:  []string{"id"},
						IsUnique: true,
					},
				},
				ForeignKeys: []*core.ForeignKey{},
				RowCount:    10000,
				UpdatedAt:   time.Now().Add(-10 * time.Minute),
			},
		},
		Views: []*core.ViewInfo{},
	}
}

func TestNewSearchEngine(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	logger := &MockLogger{}

	engine := NewSearchEngine(schema, analyzer, logger)

	assert.NotNil(t, engine)
	assert.Equal(t, schema, engine.schema)
	assert.Equal(t, analyzer, engine.analyzer)
	assert.Equal(t, logger, engine.logger)
}

func TestSearchEngine_Search_EmptyQuery(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	request := &SearchRequest{
		Query: "",
		Type:  "all",
		Limit: 10,
	}

	response, err := engine.Search(request)

	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "搜索查询不能为空")
}

func TestSearchEngine_Search_Tables(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	request := &SearchRequest{
		Query: "user",
		Type:  "table",
		Limit: 10,
	}

	response, err := engine.Search(request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Greater(t, len(response.Results), 0)
	assert.Equal(t, "user", response.Query)

	// 验证结果类型
	for _, result := range response.Results {
		assert.Equal(t, "table", result.Type)
	}

	// 应该找到 users 表
	found := false
	for _, result := range response.Results {
		if result.Name == "users" {
			found = true
			assert.Greater(t, result.Score, 0.0)
			assert.NotEmpty(t, result.Description)
			break
		}
	}
	assert.True(t, found, "应该找到 users 表")
}

func TestSearchEngine_Search_Columns(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	request := &SearchRequest{
		Query: "email",
		Type:  "column",
		Limit: 10,
	}

	response, err := engine.Search(request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Greater(t, len(response.Results), 0)

	// 验证结果类型
	for _, result := range response.Results {
		assert.Equal(t, "column", result.Type)
		assert.NotEmpty(t, result.TableName)
	}

	// 应该找到 email 列
	found := false
	for _, result := range response.Results {
		if result.Name == "email" && result.TableName == "users" {
			found = true
			assert.Greater(t, result.Score, 0.0)
			assert.Contains(t, result.Description, "varchar(100)")
			break
		}
	}
	assert.True(t, found, "应该找到 users.email 列")
}

func TestSearchEngine_Search_Indexes(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	request := &SearchRequest{
		Query: "idx_username",
		Type:  "index",
		Limit: 10,
	}

	response, err := engine.Search(request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Greater(t, len(response.Results), 0)

	// 验证结果类型
	for _, result := range response.Results {
		assert.Equal(t, "index", result.Type)
		assert.NotEmpty(t, result.TableName)
	}

	// 应该找到 idx_username 索引
	found := false
	for _, result := range response.Results {
		if result.Name == "idx_username" && result.TableName == "users" {
			found = true
			assert.Greater(t, result.Score, 0.0)
			assert.Contains(t, result.Description, "username")
			break
		}
	}
	assert.True(t, found, "应该找到 idx_username 索引")
}

func TestSearchEngine_Search_All(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	request := &SearchRequest{
		Query: "id",
		Type:  "all",
		Limit: 20,
	}

	response, err := engine.Search(request)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Greater(t, len(response.Results), 0)

	// 应该包含不同类型的结果
	hasColumn := false

	for _, result := range response.Results {
		if result.Type == "column" {
			hasColumn = true
			break
		}
	}

	assert.True(t, hasColumn, "应该包含列结果")
	// 表和索引结果可能存在也可能不存在，取决于搜索词
}

func TestSearchEngine_Search_WithFilters(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	request := &SearchRequest{
		Query: "table",
		Type:  "table",
		Limit: 10,
		Filters: map[string]string{
			"engine":   "InnoDB",
			"min_rows": "2000",
		},
	}

	response, err := engine.Search(request)

	assert.NoError(t, err)
	assert.NotNil(t, response)

	// 验证过滤器效果
	for _, result := range response.Results {
		if result.Type == "table" {
			table := result.Metadata.(*core.TableInfo)
			assert.Equal(t, "InnoDB", table.Engine)
			assert.GreaterOrEqual(t, table.RowCount, int64(2000))
		}
	}
}

func TestSearchEngine_calculateTableScore(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	table := schema.Tables[0] // users table
	options := &SearchOptions{
		FuzzyMatch:      true,
		IncludeComments: true,
		MinScore:        0.1,
	}

	// 测试精确匹配
	score := engine.calculateTableScore(table, "users", options)
	assert.Greater(t, score, 9.0) // 应该得到高分

	// 测试前缀匹配
	score = engine.calculateTableScore(table, "user", options)
	assert.Greater(t, score, 7.0) // 应该得到较高分

	// 测试包含匹配
	score = engine.calculateTableScore(table, "ser", options)
	assert.Greater(t, score, 4.0) // 应该得到中等分

	// 测试注释匹配
	score = engine.calculateTableScore(table, "用户", options)
	assert.Greater(t, score, 2.0) // 应该得到一些分数

	// 测试无匹配
	score = engine.calculateTableScore(table, "nonexistent", options)
	assert.Equal(t, 0.0, score) // 应该得到0分
}

func TestSearchEngine_calculateColumnScore(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	table := schema.Tables[0]  // users table
	column := table.Columns[1] // username column
	options := &SearchOptions{
		FuzzyMatch:      true,
		IncludeComments: true,
		MinScore:        0.1,
	}

	// 测试精确匹配
	score := engine.calculateColumnScore(column, table, "username", options)
	assert.Greater(t, score, 9.0) // 应该得到高分

	// 测试类型匹配
	score = engine.calculateColumnScore(column, table, "varchar", options)
	assert.Greater(t, score, 2.0) // 应该得到一些分数

	// 测试注释匹配
	score = engine.calculateColumnScore(column, table, "用户名", options)
	assert.Greater(t, score, 2.0) // 应该得到一些分数
}

func TestSearchEngine_calculateFuzzyScore(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	// 测试相同字符串
	score := engine.calculateFuzzyScore("users", "users")
	assert.Equal(t, 1.0, score)

	// 测试相似字符串
	score = engine.calculateFuzzyScore("users", "user")
	assert.Greater(t, score, 0.6)

	// 测试不相似字符串
	score = engine.calculateFuzzyScore("users", "posts")
	assert.Equal(t, 0.0, score) // 相似度低于阈值

	// 测试空字符串
	score = engine.calculateFuzzyScore("", "users")
	assert.Equal(t, 0.0, score)

	score = engine.calculateFuzzyScore("users", "")
	assert.Equal(t, 0.0, score)
}

func TestSearchEngine_levenshteinDistance(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abc", 0},
		{"abc", "ab", 1},
		{"abc", "def", 3},
		{"kitten", "sitting", 3},
	}

	for _, test := range tests {
		distance := engine.levenshteinDistance(test.s1, test.s2)
		assert.Equal(t, test.expected, distance, "levenshteinDistance(%q, %q)", test.s1, test.s2)
	}
}

func TestSearchEngine_highlightText(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	// 测试高亮
	result := engine.highlightText("username", "user")
	assert.Contains(t, result, "<mark>user</mark>")

	// 测试无匹配
	result = engine.highlightText("username", "xyz")
	assert.Equal(t, "username", result)

	// 测试空查询
	result = engine.highlightText("username", "")
	assert.Equal(t, "username", result)
}

func TestSearchEngine_buildTableDescription(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	table := schema.Tables[0] // users table
	description := engine.buildTableDescription(table)

	assert.Contains(t, description, "用户表")
	assert.Contains(t, description, "4 列")
	assert.Contains(t, description, "1.0K 行")
	assert.Contains(t, description, "InnoDB")
}

func TestSearchEngine_buildColumnDescription(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	table := schema.Tables[0]  // users table
	column := table.Columns[0] // id column
	description := engine.buildColumnDescription(column, table)

	assert.Contains(t, description, "bigint")
	assert.Contains(t, description, "主键")
	assert.Contains(t, description, "用户ID")
}

func TestSearchEngine_passesFilters(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	table := schema.Tables[0] // users table

	// 测试无过滤器
	assert.True(t, engine.passesFilters(table, nil))
	assert.True(t, engine.passesFilters(table, map[string]string{}))

	// 测试引擎过滤器
	assert.True(t, engine.passesFilters(table, map[string]string{"engine": "InnoDB"}))
	assert.False(t, engine.passesFilters(table, map[string]string{"engine": "MyISAM"}))

	// 测试行数过滤器
	assert.True(t, engine.passesFilters(table, map[string]string{"min_rows": "500"}))
	assert.False(t, engine.passesFilters(table, map[string]string{"min_rows": "2000"}))

	assert.True(t, engine.passesFilters(table, map[string]string{"max_rows": "2000"}))
	assert.False(t, engine.passesFilters(table, map[string]string{"max_rows": "500"}))
}

func TestSearchEngine_sortResults(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	results := []*SearchResult{
		{Name: "users", Score: 8.0, Type: "table", Metadata: schema.Tables[0]},
		{Name: "posts", Score: 9.0, Type: "table", Metadata: schema.Tables[1]},
		{Name: "comments", Score: 7.0, Type: "table", Metadata: schema.Tables[2]},
	}

	// 测试按相关性排序（默认）
	options := &SearchOptions{SortBy: "relevance", SortOrder: "desc"}
	engine.sortResults(results, options)
	assert.Equal(t, "posts", results[0].Name)
	assert.Equal(t, "users", results[1].Name)
	assert.Equal(t, "comments", results[2].Name)

	// 测试按名称排序
	options = &SearchOptions{SortBy: "name", SortOrder: "asc"}
	engine.sortResults(results, options)
	assert.Equal(t, "comments", results[0].Name)
	assert.Equal(t, "posts", results[1].Name)
	assert.Equal(t, "users", results[2].Name)

	// 测试按大小排序
	options = &SearchOptions{SortBy: "size", SortOrder: "desc"}
	engine.sortResults(results, options)
	assert.Equal(t, "comments", results[0].Name) // 10000 rows
	assert.Equal(t, "posts", results[1].Name)    // 5000 rows
	assert.Equal(t, "users", results[2].Name)    // 1000 rows
}

func TestSearchEngine_GetRecommendations(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	// 测试热门推荐
	recommendations, err := engine.GetRecommendations("popular")
	assert.NoError(t, err)
	assert.Greater(t, len(recommendations), 0)

	// 测试最近修改推荐
	recommendations, err = engine.GetRecommendations("recent")
	assert.NoError(t, err)
	assert.Greater(t, len(recommendations), 0)

	// 测试大表推荐
	recommendations, err = engine.GetRecommendations("large")
	assert.NoError(t, err)
	assert.Greater(t, len(recommendations), 0)

	// 测试默认推荐
	recommendations, err = engine.GetRecommendations("unknown")
	assert.NoError(t, err)
	assert.Greater(t, len(recommendations), 0)
}

func TestSearchEngine_AnalyzeQuery(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	// 测试搜索意图
	analysis, err := engine.AnalyzeQuery("find users")
	assert.NoError(t, err)
	assert.Equal(t, "search", analysis.Intent)

	// 测试描述意图
	analysis, err = engine.AnalyzeQuery("describe users table")
	assert.NoError(t, err)
	assert.Equal(t, "describe", analysis.Intent)

	// 测试比较意图
	analysis, err = engine.AnalyzeQuery("compare users vs posts")
	assert.NoError(t, err)
	assert.Equal(t, "compare", analysis.Intent)

	// 测试列表意图
	analysis, err = engine.AnalyzeQuery("list all tables")
	assert.NoError(t, err)
	assert.Equal(t, "list", analysis.Intent)
}

func TestSearchEngine_extractEntities(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	// 测试提取表名
	entities := engine.extractEntities("find users table")
	assert.Contains(t, entities, "users")

	// 测试提取列名
	entities = engine.extractEntities("show username and email")
	assert.Contains(t, entities, "users.username")
	assert.Contains(t, entities, "users.email")

	// 测试无实体
	entities = engine.extractEntities("some random text")
	assert.Len(t, entities, 0)
}

func TestSearchEngine_detectIntent(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	tests := []struct {
		query    string
		expected string
	}{
		{"find users", "search"},
		{"describe users", "describe"},
		{"what is users table", "describe"},
		{"compare users and posts", "compare"},
		{"users vs posts", "compare"},
		{"list all tables", "list"},
		{"show all columns", "list"},
		{"random query", "search"},
	}

	for _, test := range tests {
		intent := engine.detectIntent(test.query)
		assert.Equal(t, test.expected, intent, "Query: %s", test.query)
	}
}

func TestSearchEngine_formatNumber(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
	}

	for _, test := range tests {
		result := engine.formatNumber(test.input)
		assert.Equal(t, test.expected, result, "formatNumber(%d)", test.input)
	}
}

func TestSearchEngine_countTotalColumns(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	count := engine.countTotalColumns()
	expected := 4 + 6 + 4 // users + posts + comments
	assert.Equal(t, expected, count)
}

func TestSearchEngine_countTotalIndexes(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	count := engine.countTotalIndexes()
	expected := 3 + 3 + 1 // users + posts + comments
	assert.Equal(t, expected, count)
}

func TestSearchEngine_deduplicateStrings(t *testing.T) {
	schema := createTestSchemaForSearch()
	analyzer := NewRelationshipAnalyzer(schema, &MockLogger{})
	engine := NewSearchEngine(schema, analyzer, &MockLogger{})

	input := []string{"a", "b", "a", "c", "b", "d"}
	result := engine.deduplicateStrings(input)

	assert.Len(t, result, 4)
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "b")
	assert.Contains(t, result, "c")
	assert.Contains(t, result, "d")
}

func TestSearchEngine_containsDigits(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"abc", false},
		{"abc123", true},
		{"123abc", true},
		{"ab1c", true},
		{"", false},
		{"!@#", false},
	}

	for _, test := range tests {
		result := containsDigits(test.input)
		assert.Equal(t, test.expected, result, "containsDigits(%q)", test.input)
	}
}
