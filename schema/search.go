// 本文件实现了 Schema 搜索引擎，支持对数据库结构（表、列等）进行智能搜��和相关性分析。
// 主要功能：
// 1. 支持表、列、全局等多种类型的搜索请求。
// 2. 根据查询字符串和类型，返回相关的表、列、关系等信息。
// 3. 集成关系分析器，提升搜索结果的智能性和准确性。
// 4. 支持搜索请求参数扩展，便于后续功能拓展。
// 5. 结构体��方法均有详细注释，便于理解和维护。

package schema

import (
	"fmt" // 格式化输出
	"github.com/Anniext/rag/core"
	"math"    // 数学运算
	"sort"    // 排序
	"strconv" // 字符串与数字转换
	"strings" // 字符串处理
	"unicode" // Unicode 字符处理
)

// SearchEngine Schema 搜索引擎，负责数据库结构的智能搜索。
type SearchEngine struct {
	schema   *core.SchemaInfo      // 数据库结构信息
	analyzer *RelationshipAnalyzer // 表关系分析器
	logger   core.Logger           // 日志记录器
}

// NewSearchEngine 创建 Schema 搜索引擎。
// 参数：schema 数据库结构，analyzer 关系分析器，logger 日志记录器。
// 返回：SearchEngine 实例。
func NewSearchEngine(schema *core.SchemaInfo, analyzer *RelationshipAnalyzer, logger core.Logger) *SearchEngine {
	return &SearchEngine{
		schema:   schema,
		analyzer: analyzer,
		logger:   logger,
	}
}

// SearchRequest 搜索请求结构体，定义搜索参数。
type SearchRequest struct {
	Query   string            `json:"query"`   // 搜索关键词
	Type    string            `json:"type"`    // 搜索类型：table, column, all
	Limit   int               `json:"limit"`   // 返回结果限制数量
	Filters map[string]string `json:"filters"` // 过滤条件：引擎、字符集等
	Options *SearchOptions    `json:"options"` // 搜索选项
}

// SearchOptions 搜索选项结构体，定义模糊匹配、排序等参数。
type SearchOptions struct {
	FuzzyMatch      bool    `json:"fuzzy_match"`      // 是否启用模糊匹配
	IncludeComments bool    `json:"include_comments"` // 是否包含注释
	MinScore        float64 `json:"min_score"`        // 最小分数阈值
	SortBy          string  `json:"sort_by"`          // 排序字段：relevance, name, size
	SortOrder       string  `json:"sort_order"`       // 排序顺序：asc, desc
}

// SearchResult 搜索结果结构体，表示单个搜索结果。
type SearchResult struct {
	Type        string      `json:"type"`                 // 结果类型：table, column
	Name        string      `json:"name"`                 // 名称
	TableName   string      `json:"table_name,omitempty"` // 表名（可选）
	Description string      `json:"description"`          // 描述
	Score       float64     `json:"score"`                // 相关性分数
	Metadata    interface{} `json:"metadata"`             // 元数据
	Highlights  []string    `json:"highlights"`           // 高亮字段
}

// SearchResponse 搜索响应结构体，封装搜索结果和元数据。
type SearchResponse struct {
	Results     []*SearchResult `json:"results"`     // 搜索结果列表
	Total       int             `json:"total"`       // 结果总数
	Query       string          `json:"query"`       // 原始查询
	Suggestions []string        `json:"suggestions"` // 搜索建议
	Metadata    *SearchMetadata `json:"metadata"`    // 元数据
}

// SearchMetadata 搜索元数据结构体，包含执行时间、搜索类型等信息。
type SearchMetadata struct {
	ExecutionTime string            `json:"execution_time"` // 执行时间
	SearchType    string            `json:"search_type"`    // 搜索类型
	Filters       map[string]string `json:"filters"`        // 过滤器
	Stats         *SearchStats      `json:"stats"`          // 统计信息
}

// SearchStats 搜索统计结构体，包含搜索过程中涉及的表、列、索引数量。
type SearchStats struct {
	TablesSearched  int `json:"tables_searched"`  // 搜索的表数量
	ColumnsSearched int `json:"columns_searched"` // 搜索的列数量
	IndexesSearched int `json:"indexes_searched"` // 搜索的索引数量
}

// Search 执行搜索，根据请求类型调用不同的搜索方法。
func (s *SearchEngine) Search(request *SearchRequest) (*SearchResponse, error) {
	if request.Query == "" {
		return nil, fmt.Errorf("搜索查询不能为空")
	}

	// 设置默认值
	if request.Limit <= 0 {
		request.Limit = 20
	}
	if request.Options == nil {
		request.Options = &SearchOptions{
			FuzzyMatch:      true,
			IncludeComments: true,
			MinScore:        0.1,
			SortBy:          "relevance",
			SortOrder:       "desc",
		}
	}

	var results []*SearchResult
	stats := &SearchStats{}

	// 根据搜索类型执行搜索
	switch request.Type {
	case "table":
		tableResults := s.searchTables(request)
		results = append(results, tableResults...)
		stats.TablesSearched = len(s.schema.Tables)
	case "column":
		columnResults := s.searchColumns(request)
		results = append(results, columnResults...)
		stats.ColumnsSearched = s.countTotalColumns()
	case "index":
		indexResults := s.searchIndexes(request)
		results = append(results, indexResults...)
		stats.IndexesSearched = s.countTotalIndexes()
	default: // "all" or empty
		tableResults := s.searchTables(request)
		columnResults := s.searchColumns(request)
		indexResults := s.searchIndexes(request)
		results = append(results, tableResults...)
		results = append(results, columnResults...)
		results = append(results, indexResults...)
		stats.TablesSearched = len(s.schema.Tables)
		stats.ColumnsSearched = s.countTotalColumns()
		stats.IndexesSearched = s.countTotalIndexes()
	}

	// 过滤低分结果
	filteredResults := make([]*SearchResult, 0)
	for _, result := range results {
		if result.Score >= request.Options.MinScore {
			filteredResults = append(filteredResults, result)
		}
	}

	// 排序
	s.sortResults(filteredResults, request.Options)

	// 限制结果数量
	if len(filteredResults) > request.Limit {
		filteredResults = filteredResults[:request.Limit]
	}

	// 生成建议
	suggestions := s.generateSuggestions(request.Query, filteredResults)

	response := &SearchResponse{
		Results:     filteredResults,
		Total:       len(filteredResults),
		Query:       request.Query,
		Suggestions: suggestions,
		Metadata: &SearchMetadata{
			SearchType: request.Type,
			Filters:    request.Filters,
			Stats:      stats,
		},
	}

	return response, nil
}

// searchTables 搜索表，遍历所有表并计算相关性分数。
func (s *SearchEngine) searchTables(request *SearchRequest) []*SearchResult {
	var results []*SearchResult
	query := strings.ToLower(request.Query)

	for _, table := range s.schema.Tables {
		score := s.calculateTableScore(table, query, request.Options)
		if score > 0 {
			highlights := s.generateTableHighlights(table, query)

			result := &SearchResult{
				Type:        "table",
				Name:        table.Name,
				Description: s.buildTableDescription(table),
				Score:       score,
				Metadata:    table,
				Highlights:  highlights,
			}

			// 应用过滤器
			if s.passesFilters(table, request.Filters) {
				results = append(results, result)
			}
		}
	}

	return results
}

// searchColumns 搜索列，遍历所有列并计算相关性分数。
func (s *SearchEngine) searchColumns(request *SearchRequest) []*SearchResult {
	var results []*SearchResult
	query := strings.ToLower(request.Query)

	for _, table := range s.schema.Tables {
		for _, column := range table.Columns {
			score := s.calculateColumnScore(column, table, query, request.Options)
			if score > 0 {
				highlights := s.generateColumnHighlights(column, query)

				result := &SearchResult{
					Type:        "column",
					Name:        column.Name,
					TableName:   table.Name,
					Description: s.buildColumnDescription(column, table),
					Score:       score,
					Metadata:    column,
					Highlights:  highlights,
				}

				results = append(results, result)
			}
		}
	}

	return results
}

// searchIndexes 搜索索引，遍历所有索引并计算相关性分数。
func (s *SearchEngine) searchIndexes(request *SearchRequest) []*SearchResult {
	var results []*SearchResult
	query := strings.ToLower(request.Query)

	for _, table := range s.schema.Tables {
		for _, index := range table.Indexes {
			score := s.calculateIndexScore(index, table, query, request.Options)
			if score > 0 {
				highlights := s.generateIndexHighlights(index, query)

				result := &SearchResult{
					Type:        "index",
					Name:        index.Name,
					TableName:   table.Name,
					Description: s.buildIndexDescription(index, table),
					Score:       score,
					Metadata:    index,
					Highlights:  highlights,
				}

				results = append(results, result)
			}
		}
	}

	return results
}

// calculateTableScore 计算表的相关性分数，考虑名称、注释、模糊匹配等因素。
func (s *SearchEngine) calculateTableScore(table *core.TableInfo, query string, options *SearchOptions) float64 {
	var score float64

	tableName := strings.ToLower(table.Name)
	tableComment := strings.ToLower(table.Comment)

	// 精确匹配
	if tableName == query {
		score += 10.0
	}

	// 前缀匹配
	if strings.HasPrefix(tableName, query) {
		score += 8.0
	}

	// 包含匹配
	if strings.Contains(tableName, query) {
		score += 5.0
	}

	// 注释匹配
	if options.IncludeComments && strings.Contains(tableComment, query) {
		score += 3.0
	}

	// 模糊匹配
	if options.FuzzyMatch {
		fuzzyScore := s.calculateFuzzyScore(tableName, query)
		score += fuzzyScore * 2.0
	}

	// 根据表的重要性调整分数
	score *= s.calculateTableImportance(table)

	return score
}

// calculateColumnScore 计算列的相关性分数，考虑名称、类型、注释等因素。
func (s *SearchEngine) calculateColumnScore(column *core.Column, table *core.TableInfo, query string, options *SearchOptions) float64 {
	var score float64

	columnName := strings.ToLower(column.Name)
	columnComment := strings.ToLower(column.Comment)
	columnType := strings.ToLower(column.Type)

	// 精确匹配
	if columnName == query {
		score += 10.0
	}

	// 前缀匹配
	if strings.HasPrefix(columnName, query) {
		score += 8.0
	}

	// 包含匹配
	if strings.Contains(columnName, query) {
		score += 5.0
	}

	// 类型匹配
	if strings.Contains(columnType, query) {
		score += 3.0
	}

	// 注释匹配
	if options.IncludeComments && strings.Contains(columnComment, query) {
		score += 3.0
	}

	// 模糊匹配
	if options.FuzzyMatch {
		fuzzyScore := s.calculateFuzzyScore(columnName, query)
		score += fuzzyScore * 2.0
	}

	// 根据列的重要性调整分数
	score *= s.calculateColumnImportance(column)

	return score
}

// calculateIndexScore 计算索引的相关性分数，考虑名称、列名等因素。
func (s *SearchEngine) calculateIndexScore(index *core.Index, table *core.TableInfo, query string, options *SearchOptions) float64 {
	var score float64

	indexName := strings.ToLower(index.Name)

	// 精确匹配
	if indexName == query {
		score += 10.0
	}

	// 前缀匹配
	if strings.HasPrefix(indexName, query) {
		score += 8.0
	}

	// 包含匹配
	if strings.Contains(indexName, query) {
		score += 5.0
	}

	// 列名匹配
	for _, column := range index.Columns {
		if strings.Contains(strings.ToLower(column), query) {
			score += 4.0
		}
	}

	// 模糊匹配
	if options.FuzzyMatch {
		fuzzyScore := s.calculateFuzzyScore(indexName, query)
		score += fuzzyScore * 2.0
	}

	// 根据索引类型调整分数
	switch index.Type {
	case "PRIMARY":
		score *= 1.5
	case "UNIQUE":
		score *= 1.3
	case "FULLTEXT":
		score *= 1.2
	}

	return score
}

// calculateFuzzyScore 计算模糊匹配分数，使用编辑距离算法。
func (s *SearchEngine) calculateFuzzyScore(text, query string) float64 {
	if len(query) == 0 || len(text) == 0 {
		return 0
	}

	// 使用编辑距离计算相似度
	distance := s.levenshteinDistance(text, query)
	maxLen := math.Max(float64(len(text)), float64(len(query)))

	if maxLen == 0 {
		return 1.0
	}

	similarity := 1.0 - (float64(distance) / maxLen)

	// 只有相似度超过阈值才返回分数
	if similarity > 0.6 {
		return similarity
	}

	return 0
}

// levenshteinDistance 计算编辑距离，返回两个字符串之间的编辑操作最小次数。
func (s *SearchEngine) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				min(matrix[i-1][j]+1, matrix[i][j-1]+1), // deletion, insertion
				matrix[i-1][j-1]+cost,                   // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// calculateTableImportance 计算表的重要性，考虑行数、外键数量等因素。
func (s *SearchEngine) calculateTableImportance(table *core.TableInfo) float64 {
	importance := 1.0

	// 根据表大小调整
	if table.RowCount > 10000 {
		importance *= 1.2
	} else if table.RowCount < 100 {
		importance *= 0.8
	}

	// 根据外键���量调整（关联性强的表更重要）
	if len(table.ForeignKeys) > 2 {
		importance *= 1.1
	}

	// 根据索引数量调整
	if len(table.Indexes) > 3 {
		importance *= 1.1
	}

	return importance
}

// calculateColumnImportance 计算列的重要性，考虑主键、自增、非空等属性。
func (s *SearchEngine) calculateColumnImportance(column *core.Column) float64 {
	importance := 1.0

	// 主键更重要
	if column.IsPrimaryKey {
		importance *= 1.5
	}

	// 自增列更重要
	if column.IsAutoIncrement {
		importance *= 1.2
	}

	// 非空列更重要
	if !column.Nullable {
		importance *= 1.1
	}

	return importance
}

// generateTableHighlights 生成表的高亮信息，便于前端展示。
func (s *SearchEngine) generateTableHighlights(table *core.TableInfo, query string) []string {
	var highlights []string

	tableName := strings.ToLower(table.Name)
	tableComment := strings.ToLower(table.Comment)

	if strings.Contains(tableName, query) {
		highlights = append(highlights, fmt.Sprintf("表名: %s", s.highlightText(table.Name, query)))
	}

	if strings.Contains(tableComment, query) {
		highlights = append(highlights, fmt.Sprintf("注释: %s", s.highlightText(table.Comment, query)))
	}

	return highlights
}

// generateColumnHighlights 生成列的高亮信息，便于前端展示。
func (s *SearchEngine) generateColumnHighlights(column *core.Column, query string) []string {
	var highlights []string

	columnName := strings.ToLower(column.Name)
	columnComment := strings.ToLower(column.Comment)
	columnType := strings.ToLower(column.Type)

	if strings.Contains(columnName, query) {
		highlights = append(highlights, fmt.Sprintf("列名: %s", s.highlightText(column.Name, query)))
	}

	if strings.Contains(columnType, query) {
		highlights = append(highlights, fmt.Sprintf("类型: %s", s.highlightText(column.Type, query)))
	}

	if strings.Contains(columnComment, query) {
		highlights = append(highlights, fmt.Sprintf("注释: %s", s.highlightText(column.Comment, query)))
	}

	return highlights
}

// generateIndexHighlights 生成索引的高亮信息，便于前端展示。
func (s *SearchEngine) generateIndexHighlights(index *core.Index, query string) []string {
	var highlights []string

	indexName := strings.ToLower(index.Name)

	if strings.Contains(indexName, query) {
		highlights = append(highlights, fmt.Sprintf("索引名: %s", s.highlightText(index.Name, query)))
	}

	for _, column := range index.Columns {
		if strings.Contains(strings.ToLower(column), query) {
			highlights = append(highlights, fmt.Sprintf("列: %s", s.highlightText(column, query)))
		}
	}

	return highlights
}

// highlightText 高亮文本，简单的高亮实现。
func (s *SearchEngine) highlightText(text, query string) string {
	if query == "" {
		return text
	}

	// 简单的高亮实现，在实际应用中可能需要更复杂的逻辑
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)

	if strings.Contains(lowerText, lowerQuery) {
		// 找到匹配位置
		index := strings.Index(lowerText, lowerQuery)
		if index >= 0 {
			before := text[:index]
			match := text[index : index+len(query)]
			after := text[index+len(query):]
			return fmt.Sprintf("%s<mark>%s</mark>%s", before, match, after)
		}
	}

	return text
}

// buildTableDescription 构建表描述，包含注释、列数、行数等信息。
func (s *SearchEngine) buildTableDescription(table *core.TableInfo) string {
	parts := []string{}

	if table.Comment != "" {
		parts = append(parts, table.Comment)
	}

	parts = append(parts, fmt.Sprintf("%d 列", len(table.Columns)))

	if table.RowCount > 0 {
		parts = append(parts, fmt.Sprintf("%s 行", s.formatNumber(table.RowCount)))
	}

	if table.Engine != "" {
		parts = append(parts, fmt.Sprintf("引擎: %s", table.Engine))
	}

	return strings.Join(parts, " | ")
}

// buildColumnDescription 构建列描述，包含类型、注释、主键等信息。
func (s *SearchEngine) buildColumnDescription(column *core.Column, table *core.TableInfo) string {
	parts := []string{}

	parts = append(parts, fmt.Sprintf("类型: %s", column.Type))

	if column.Comment != "" {
		parts = append(parts, column.Comment)
	}

	if column.IsPrimaryKey {
		parts = append(parts, "主键")
	}

	if column.IsAutoIncrement {
		parts = append(parts, "自增")
	}

	if !column.Nullable {
		parts = append(parts, "非空")
	}

	if column.DefaultValue != "" {
		parts = append(parts, fmt.Sprintf("默认值: %s", column.DefaultValue))
	}

	return strings.Join(parts, " | ")
}

// buildIndexDescription 构建索引描述，包含类型、列名等信息。
func (s *SearchEngine) buildIndexDescription(index *core.Index, table *core.TableInfo) string {
	parts := []string{}

	parts = append(parts, fmt.Sprintf("类型: %s", index.Type))
	parts = append(parts, fmt.Sprintf("列: %s", strings.Join(index.Columns, ", ")))

	if index.IsUnique {
		parts = append(parts, "唯一")
	}

	return strings.Join(parts, " | ")
}

// passesFilters 检查是否通过过滤器，过滤器包括引擎、字符集、行数范围等。
func (s *SearchEngine) passesFilters(table *core.TableInfo, filters map[string]string) bool {
	if filters == nil {
		return true
	}

	for key, value := range filters {
		switch key {
		case "engine":
			if !strings.EqualFold(table.Engine, value) {
				return false
			}
		case "charset":
			if !strings.EqualFold(table.Charset, value) {
				return false
			}
		case "min_rows":
			if minRows, err := strconv.ParseInt(value, 10, 64); err == nil {
				if table.RowCount < minRows {
					return false
				}
			}
		case "max_rows":
			if maxRows, err := strconv.ParseInt(value, 10, 64); err == nil {
				if table.RowCount > maxRows {
					return false
				}
			}
		}
	}

	return true
}

// sortResults 排序结果，根据选项中的排序字段和顺序进行排序。
func (s *SearchEngine) sortResults(results []*SearchResult, options *SearchOptions) {
	sort.Slice(results, func(i, j int) bool {
		switch options.SortBy {
		case "name":
			if options.SortOrder == "asc" {
				return results[i].Name < results[j].Name
			}
			return results[i].Name > results[j].Name
		case "size":
			// 只对表类型有效
			if results[i].Type == "table" && results[j].Type == "table" {
				table1 := results[i].Metadata.(*core.TableInfo)
				table2 := results[j].Metadata.(*core.TableInfo)
				if options.SortOrder == "asc" {
					return table1.RowCount < table2.RowCount
				}
				return table1.RowCount > table2.RowCount
			}
			fallthrough
		default: // "relevance"
			if options.SortOrder == "asc" {
				return results[i].Score < results[j].Score
			}
			return results[i].Score > results[j].Score
		}
	})
}

// generateSuggestions 生成搜索建议，基于查询词和现有结果。
func (s *SearchEngine) generateSuggestions(query string, results []*SearchResult) []string {
	var suggestions []string

	// 如果结果很少，提供相似的建议
	if len(results) < 3 {
		suggestions = append(suggestions, s.generateSimilarTerms(query)...)
	}

	// 基于现有结果生成建议
	if len(results) > 0 {
		suggestions = append(suggestions, s.generateRelatedSuggestions(results)...)
	}

	// 去重并限制数量
	suggestions = s.deduplicateStrings(suggestions)
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}

	return suggestions
}

// generateSimilarTerms 生成相似术语，基于常见的数据库术语。
func (s *SearchEngine) generateSimilarTerms(query string) []string {
	var suggestions []string

	// 基于常见的数据库术语生成建议
	commonTerms := []string{
		"id", "name", "title", "description", "content", "status", "type",
		"created_at", "updated_at", "deleted_at", "user_id", "email",
		"password", "token", "key", "value", "data", "info", "config",
	}

	for _, term := range commonTerms {
		if s.calculateFuzzyScore(term, query) > 0.5 {
			suggestions = append(suggestions, term)
		}
	}

	return suggestions
}

// generateRelatedSuggestions 基于结果生成相关建议，从表的列中提取建议。
func (s *SearchEngine) generateRelatedSuggestions(results []*SearchResult) []string {
	var suggestions []string

	// 从结果中提取相关术语
	for _, result := range results {
		if result.Type == "table" {
			table := result.Metadata.(*core.TableInfo)
			for _, column := range table.Columns {
				if len(suggestions) < 10 {
					suggestions = append(suggestions, column.Name)
				}
			}
		}
	}

	return suggestions
}

// deduplicateStrings 去重字符串切片，返回不重复的字符串列表。
func (s *SearchEngine) deduplicateStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, str := range strs {
		if !seen[str] {
			seen[str] = true
			result = append(result, str)
		}
	}

	return result
}

// formatNumber 格式化数字，支持 K 和 M 级别的单位。
func (s *SearchEngine) formatNumber(num int64) string {
	if num < 1000 {
		return strconv.FormatInt(num, 10)
	} else if num < 1000000 {
		return fmt.Sprintf("%.1fK", float64(num)/1000)
	} else {
		return fmt.Sprintf("%.1fM", float64(num)/1000000)
	}
}

// countTotalColumns 计算总列数，遍历所有表的列。
func (s *SearchEngine) countTotalColumns() int {
	count := 0
	for _, table := range s.schema.Tables {
		count += len(table.Columns)
	}
	return count
}

// countTotalIndexes 计算总索引数，遍历所有表的索引。
func (s *SearchEngine) countTotalIndexes() int {
	count := 0
	for _, table := range s.schema.Tables {
		count += len(table.Indexes)
	}
	return count
}

// min 返回两个整数的最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetRecommendations 获取推荐，基于上下文提供表的推荐列表。
func (s *SearchEngine) GetRecommendations(context string) ([]*SearchResult, error) {
	var recommendations []*SearchResult

	// 基于上下文生成推荐
	switch strings.ToLower(context) {
	case "popular":
		recommendations = s.getPopularTables()
	case "recent":
		recommendations = s.getRecentlyModifiedTables()
	case "large":
		recommendations = s.getLargeTables()
	case "indexed":
		recommendations = s.getWellIndexedTables()
	default:
		recommendations = s.getPopularTables()
	}

	return recommendations, nil
}

// getPopularTables 获取热门表（基于行数和关联数），返回热门表的搜索结果。
func (s *SearchEngine) getPopularTables() []*SearchResult {
	var results []*SearchResult

	// 按行数和外键数量排序
	tables := make([]*core.TableInfo, len(s.schema.Tables))
	copy(tables, s.schema.Tables)

	sort.Slice(tables, func(i, j int) bool {
		score1 := float64(tables[i].RowCount) + float64(len(tables[i].ForeignKeys))*1000
		score2 := float64(tables[j].RowCount) + float64(len(tables[j].ForeignKeys))*1000
		return score1 > score2
	})

	for i, table := range tables {
		if i >= 10 { // 限制数量
			break
		}

		result := &SearchResult{
			Type:        "table",
			Name:        table.Name,
			Description: s.buildTableDescription(table),
			Score:       10.0 - float64(i), // 递减分数
			Metadata:    table,
		}
		results = append(results, result)
	}

	return results
}

// getRecentlyModifiedTables 获取最近修改的表，返回最近修改表的搜索结果。
func (s *SearchEngine) getRecentlyModifiedTables() []*SearchResult {
	var results []*SearchResult

	// 按更新时间排序
	tables := make([]*core.TableInfo, len(s.schema.Tables))
	copy(tables, s.schema.Tables)

	sort.Slice(tables, func(i, j int) bool {
		return tables[i].UpdatedAt.After(tables[j].UpdatedAt)
	})

	for i, table := range tables {
		if i >= 10 { // 限制数量
			break
		}

		result := &SearchResult{
			Type:        "table",
			Name:        table.Name,
			Description: s.buildTableDescription(table),
			Score:       10.0 - float64(i), // 递减分数
			Metadata:    table,
		}
		results = append(results, result)
	}

	return results
}

// getLargeTables 获取大表，返回行数较多的表的搜索结果。
func (s *SearchEngine) getLargeTables() []*SearchResult {
	var results []*SearchResult

	// 按行数排序
	tables := make([]*core.TableInfo, len(s.schema.Tables))
	copy(tables, s.schema.Tables)

	sort.Slice(tables, func(i, j int) bool {
		return tables[i].RowCount > tables[j].RowCount
	})

	for i, table := range tables {
		if i >= 10 || table.RowCount == 0 { // 限制数量或跳过空表
			break
		}

		result := &SearchResult{
			Type:        "table",
			Name:        table.Name,
			Description: s.buildTableDescription(table),
			Score:       10.0 - float64(i), // 递减分数
			Metadata:    table,
		}
		results = append(results, result)
	}

	return results
}

// getWellIndexedTables 获取索引良好的表，返回索引数量较多的表的搜索结果。
func (s *SearchEngine) getWellIndexedTables() []*SearchResult {
	var results []*SearchResult

	// 按索引数量排序
	tables := make([]*core.TableInfo, len(s.schema.Tables))
	copy(tables, s.schema.Tables)

	sort.Slice(tables, func(i, j int) bool {
		return len(tables[i].Indexes) > len(tables[j].Indexes)
	})

	for i, table := range tables {
		if i >= 10 || len(table.Indexes) <= 1 { // 限制数量或跳过只有主键的表
			break
		}

		result := &SearchResult{
			Type:        "table",
			Name:        table.Name,
			Description: s.buildTableDescription(table),
			Score:       10.0 - float64(i), // 递减分数
			Metadata:    table,
		}
		results = append(results, result)
	}

	return results
}

// AnalyzeQuery 分析查询意图，返回查询分析结果。
func (s *SearchEngine) AnalyzeQuery(query string) (*QueryAnalysis, error) {
	analysis := &QueryAnalysis{
		Query:       query,
		Intent:      s.detectIntent(query),
		Entities:    s.extractEntities(query),
		Suggestions: s.generateQuerySuggestions(query),
	}

	return analysis, nil
}

// QueryAnalysis 查询分析结果结构体，包含查询意图、实体和建议。
type QueryAnalysis struct {
	Query       string   `json:"query"`       // 原始查询
	Intent      string   `json:"intent"`      // search, describe, compare, etc.
	Entities    []string `json:"entities"`    // 识别出的实体
	Suggestions []string `json:"suggestions"` // 查询建议
	Confidence  float64  `json:"confidence"`  // 置信度
}

// detectIntent 检测查询意图，返回查询的主要意图。
func (s *SearchEngine) detectIntent(query string) string {
	query = strings.ToLower(query)

	// 列表意图（优先检查，因为 "show all" 比 "show" 更具体）
	listKeywords := []string{"list", "show all", "list all", "列出", "所有", "全部"}
	for _, keyword := range listKeywords {
		if strings.Contains(query, keyword) {
			return "list"
		}
	}

	// 比较意图
	compareKeywords := []string{"compare", "vs", "difference", "比较", "对比", "区别"}
	for _, keyword := range compareKeywords {
		if strings.Contains(query, keyword) {
			return "compare"
		}
	}

	// 描述意图
	describeKeywords := []string{"describe", "desc", "show", "explain", "what is", "告诉我", "描述", "解释"}
	for _, keyword := range describeKeywords {
		if strings.Contains(query, keyword) {
			return "describe"
		}
	}

	// 默认为搜索意图
	return "search"
}

// extractEntities 提取实体，从查询中识别出表名和列名。
func (s *SearchEngine) extractEntities(query string) []string {
	var entities []string

	words := strings.Fields(strings.ToLower(query))

	// 检查是否包含已知的表名或列名
	for _, word := range words {
		// 清理标点符号
		cleanWord := strings.Trim(word, ".,!?;:")

		// 检查表名
		for _, table := range s.schema.Tables {
			if strings.EqualFold(table.Name, cleanWord) {
				entities = append(entities, table.Name)
			}

			// 检查列名
			for _, column := range table.Columns {
				if strings.EqualFold(column.Name, cleanWord) {
					entities = append(entities, fmt.Sprintf("%s.%s", table.Name, column.Name))
				}
			}
		}
	}

	return s.deduplicateStrings(entities)
}

// generateQuerySuggestions 生成查询建议，基于查询长度和内容。
func (s *SearchEngine) generateQuerySuggestions(query string) []string {
	var suggestions []string

	// 基于查询长度和内容生成建议
	if len(query) < 3 {
		suggestions = append(suggestions, "尝试输入更长的搜索词")
		suggestions = append(suggestions, "使用表名或列名进行搜索")
	}

	// 如果查询包含数字，建议搜索相关类型
	if containsDigits(query) {
		suggestions = append(suggestions, "搜索数值类型的列")
		suggestions = append(suggestions, "查找包含ID的字段")
	}

	// 如果查询包含日期相关词汇
	dateKeywords := []string{"date", "time", "created", "updated", "日期", "时间", "创建", "更新"}
	for _, keyword := range dateKeywords {
		if strings.Contains(strings.ToLower(query), keyword) {
			suggestions = append(suggestions, "搜索时间戳字段")
			suggestions = append(suggestions, "查找审计字段")
			break
		}
	}

	return suggestions
}

// containsDigits 检查字符串是否包含数字
func containsDigits(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
