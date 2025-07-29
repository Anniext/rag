// 本文件实现了数据库表关系分析器，支持自动分析显式外键、隐式命名约定、多对多关系，并生成表关系图和查询优化建议。
// 主要功能：
// 1. 自动识别表之间的各种关系（外键、命名推断、多对多中间表）。
// 2. 构建表关系图，便于后续查询优化、可视化和依赖分析。
// 3. 提供查询优化建议，包括连接顺序、索引建议、复杂度分析等。
// 4. 支持英文单复数推断、循环依赖检测等辅助功能。
// 5. 结构体和方法均有详细注释，便于理解和维护。

package schema

import (
	"fmt"
	"github.com/Anniext/rag/core"
	"sort"
	"strings"
)

// RelationshipAnalyzer 表关系分析器，负责分析 schema 中所有表的关系。
type RelationshipAnalyzer struct {
	schema *core.SchemaInfo // 数据库 schema 信息，包含所有表结构
	logger core.Logger      // 日志记录器，用于输出分析过程信息
}

// NewRelationshipAnalyzer 创建表关系分析器。
// 参数：schema 数据库结构信息，logger 日志记录器。
// 返回：RelationshipAnalyzer 实例。
func NewRelationshipAnalyzer(schema *core.SchemaInfo, logger core.Logger) *RelationshipAnalyzer {
	return &RelationshipAnalyzer{
		schema: schema,
		logger: logger,
	}
}

// AnalyzeRelationships 分析所有表之间的关系，生成关系图。
// 返回：关系图和错误信息。
func (a *RelationshipAnalyzer) AnalyzeRelationships() (*RelationshipGraph, error) {
	if a.schema == nil {
		return nil, fmt.Errorf("schema 信息为空")
	}

	graph := &RelationshipGraph{
		Tables:        make(map[string]*TableNode),   // 所有表节点
		Relationships: make([]*core.Relationship, 0), // 所有关系
	}

	// 创建表节点
	for _, table := range a.schema.Tables {
		node := &TableNode{
			Table:         table,                         // 表结构体
			IncomingRefs:  make([]*core.Relationship, 0), // 指向本表的关系
			OutgoingRefs:  make([]*core.Relationship, 0), // 本表指向其他表的关系
			RelatedTables: make(map[string]bool),         // 相关表集合
		}
		graph.Tables[table.Name] = node
	}

	// 分析显式外键关系
	if err := a.analyzeExplicitRelationships(graph); err != nil {
		return nil, fmt.Errorf("分析外键关系失败: %w", err)
	}

	// 分析隐式关系（基于命名约定）
	if err := a.analyzeImplicitRelationships(graph); err != nil {
		return nil, fmt.Errorf("分析隐式关系失败: %w", err)
	}

	// 分析多对多关系
	if err := a.analyzeManyToManyRelationships(graph); err != nil {
		return nil, fmt.Errorf("分析多对多关系失败: %w", err)
	}

	a.logger.Info("表关系分析完成",
		"tables", len(graph.Tables),
		"relationships", len(graph.Relationships))

	return graph, nil
}

// analyzeExplicitRelationships 分析显式关系（外键约束）
func (a *RelationshipAnalyzer) analyzeExplicitRelationships(graph *RelationshipGraph) error {
	for _, table := range a.schema.Tables {
		for _, fk := range table.ForeignKeys {
			// 创建关系
			relationship := &core.Relationship{
				Type:            a.determineRelationshipType(table.Name, fk),
				FromTable:       table.Name,
				FromColumn:      fk.Column,
				ToTable:         fk.ReferencedTable,
				ToColumn:        fk.ReferencedColumn,
				RelationshipKey: fk.Name,
			}

			// 添加到图中
			graph.Relationships = append(graph.Relationships, relationship)

			// 更新表节点
			if fromNode, exists := graph.Tables[table.Name]; exists {
				fromNode.OutgoingRefs = append(fromNode.OutgoingRefs, relationship)
				fromNode.RelatedTables[fk.ReferencedTable] = true
			}

			if toNode, exists := graph.Tables[fk.ReferencedTable]; exists {
				toNode.IncomingRefs = append(toNode.IncomingRefs, relationship)
				toNode.RelatedTables[table.Name] = true
			}
		}
	}

	return nil
}

// analyzeImplicitRelationships 分析隐式关系（基于命名决定）
func (a *RelationshipAnalyzer) analyzeImplicitRelationships(graph *RelationshipGraph) error {
	for _, table := range a.schema.Tables {
		for _, column := range table.Columns {
			// 跳过已有外键约束的列
			if a.hasExplicitForeignKey(table, column.Name) {
				continue
			}

			// 基于命名约定推断关系
			if referencedTable := a.inferReferencedTable(column.Name, table.Name); referencedTable != "" {
				if _, exists := graph.Tables[referencedTable]; exists {
					relationship := &core.Relationship{
						Type:            "inferred_many_to_one",
						FromTable:       table.Name,
						FromColumn:      column.Name,
						ToTable:         referencedTable,
						ToColumn:        "id", // 假设引用主键
						RelationshipKey: fmt.Sprintf("inferred_%s_%s", table.Name, column.Name),
					}

					// 检查是否已存在相同关系
					if !a.relationshipExists(graph, relationship) {
						graph.Relationships = append(graph.Relationships, relationship)

						// 更新表节点
						if fromNode, exists := graph.Tables[table.Name]; exists {
							fromNode.OutgoingRefs = append(fromNode.OutgoingRefs, relationship)
							fromNode.RelatedTables[referencedTable] = true
						}

						if toNode, exists := graph.Tables[referencedTable]; exists {
							toNode.IncomingRefs = append(toNode.IncomingRefs, relationship)
							toNode.RelatedTables[table.Name] = true
						}
					}
				}
			}
		}
	}

	return nil
}

// analyzeManyToManyRelationships 分析多对多关系
func (a *RelationshipAnalyzer) analyzeManyToManyRelationships(graph *RelationshipGraph) error {
	// 查找可能的中间表
	for _, node := range graph.Tables {
		if a.isPossibleJunctionTable(node.Table) {
			// 分析中间表的外键关系
			if len(node.Table.ForeignKeys) >= 2 {
				relationships := a.createManyToManyRelationships(node.Table)
				for _, rel := range relationships {
					graph.Relationships = append(graph.Relationships, rel)
				}
			}
		}
	}

	return nil
}

// determineRelationshipType 确定关系类型
func (a *RelationshipAnalyzer) determineRelationshipType(fromTable string, fk *core.ForeignKey) string {
	// 检查外键列是否唯一
	fromTableInfo := a.getTableByName(fromTable)
	if fromTableInfo != nil {
		for _, index := range fromTableInfo.Indexes {
			if index.IsUnique && len(index.Columns) == 1 && index.Columns[0] == fk.Column {
				return "one_to_one"
			}
		}
	}

	return "many_to_one"
}

// hasExplicitForeignKey 检查列是否有显式外键约束
func (a *RelationshipAnalyzer) hasExplicitForeignKey(table *core.TableInfo, columnName string) bool {
	for _, fk := range table.ForeignKeys {
		if fk.Column == columnName {
			return true
		}
	}
	return false
}

// inferReferencedTable 基于命名约定推断引用的表
func (a *RelationshipAnalyzer) inferReferencedTable(columnName, currentTable string) string {
	columnName = strings.ToLower(columnName)

	// 常见的外键命名模式
	patterns := []string{
		"_id",
		"_key",
		"_ref",
	}

	for _, pattern := range patterns {
		if strings.HasSuffix(columnName, pattern) {
			// 提取表名部分
			tableName := strings.TrimSuffix(columnName, pattern)

			// 尝试单数和复数形式
			candidates := []string{
				tableName,
				a.pluralize(tableName),
				a.singularize(tableName),
			}

			for _, candidate := range candidates {
				if candidate != strings.ToLower(currentTable) && a.tableExists(candidate) {
					return candidate
				}
			}
		}
	}

	return ""
}

// isPossibleJunctionTable 判断是否为可能的中间表
func (a *RelationshipAnalyzer) isPossibleJunctionTable(table *core.TableInfo) bool {
	// 中间表通常有以下特征：
	// 1. 有两个或更多外键
	// 2. 表名包含连接词（如 user_role, post_tag）
	// 3. 主要由外键列组成

	if len(table.ForeignKeys) < 2 {
		return false
	}

	// 检查表名是否包含连接模式
	tableName := strings.ToLower(table.Name)
	connectionPatterns := []string{"_", "-"}

	for _, pattern := range connectionPatterns {
		if strings.Contains(tableName, pattern) {
			parts := strings.Split(tableName, pattern)
			if len(parts) >= 2 {
				// 检查部分是否对应现有表名
				validParts := 0
				for _, part := range parts {
					if a.tableExists(part) || a.tableExists(a.pluralize(part)) {
						validParts++
					}
				}
				if validParts >= 2 {
					return true
				}
			}
		}
	}

	// 检查列组成（主要由外键组成）
	fkColumns := make(map[string]bool)
	for _, fk := range table.ForeignKeys {
		fkColumns[fk.Column] = true
	}

	nonFkColumns := 0
	for _, column := range table.Columns {
		if !fkColumns[column.Name] && !column.IsPrimaryKey {
			nonFkColumns++
		}
	}

	// 如果非外键列很少，可能是中间表
	return nonFkColumns <= 2
}

// createManyToManyRelationships 创建多对多关系
func (a *RelationshipAnalyzer) createManyToManyRelationships(junctionTable *core.TableInfo) []*core.Relationship {
	var relationships []*core.Relationship

	if len(junctionTable.ForeignKeys) >= 2 {
		// 为每对外键创建多对多关系
		for i := 0; i < len(junctionTable.ForeignKeys); i++ {
			for j := i + 1; j < len(junctionTable.ForeignKeys); j++ {
				fk1 := junctionTable.ForeignKeys[i]
				fk2 := junctionTable.ForeignKeys[j]

				// 创建双向多对多关系
				rel1 := &core.Relationship{
					Type:            "many_to_many",
					FromTable:       fk1.ReferencedTable,
					FromColumn:      fk1.ReferencedColumn,
					ToTable:         fk2.ReferencedTable,
					ToColumn:        fk2.ReferencedColumn,
					RelationshipKey: fmt.Sprintf("m2m_%s_%s_via_%s", fk1.ReferencedTable, fk2.ReferencedTable, junctionTable.Name),
				}

				rel2 := &core.Relationship{
					Type:            "many_to_many",
					FromTable:       fk2.ReferencedTable,
					FromColumn:      fk2.ReferencedColumn,
					ToTable:         fk1.ReferencedTable,
					ToColumn:        fk1.ReferencedColumn,
					RelationshipKey: fmt.Sprintf("m2m_%s_%s_via_%s", fk2.ReferencedTable, fk1.ReferencedTable, junctionTable.Name),
				}

				relationships = append(relationships, rel1, rel2)
			}
		}
	}

	return relationships
}

// relationshipExists 检查关系是否已存在
func (a *RelationshipAnalyzer) relationshipExists(graph *RelationshipGraph, newRel *core.Relationship) bool {
	for _, existing := range graph.Relationships {
		if existing.FromTable == newRel.FromTable &&
			existing.FromColumn == newRel.FromColumn &&
			existing.ToTable == newRel.ToTable &&
			existing.ToColumn == newRel.ToColumn {
			return true
		}
	}
	return false
}

// getTableByName 根据名称获取表信息
func (a *RelationshipAnalyzer) getTableByName(tableName string) *core.TableInfo {
	for _, table := range a.schema.Tables {
		if table.Name == tableName {
			return table
		}
	}
	return nil
}

// tableExists 检查表是否存在
func (a *RelationshipAnalyzer) tableExists(tableName string) bool {
	return a.getTableByName(tableName) != nil
}

// pluralize 简单的复数化（英文）
func (a *RelationshipAnalyzer) pluralize(word string) string {
	if strings.HasSuffix(word, "y") {
		return strings.TrimSuffix(word, "y") + "ies"
	}
	if strings.HasSuffix(word, "s") || strings.HasSuffix(word, "x") || strings.HasSuffix(word, "z") {
		return word + "es"
	}
	return word + "s"
}

// singularize 简单的单数化（英文）
func (a *RelationshipAnalyzer) singularize(word string) string {
	if strings.HasSuffix(word, "ies") {
		return strings.TrimSuffix(word, "ies") + "y"
	}
	if strings.HasSuffix(word, "es") {
		return strings.TrimSuffix(word, "es")
	}
	if strings.HasSuffix(word, "s") && len(word) > 1 {
		return strings.TrimSuffix(word, "s")
	}
	return word
}

// RelationshipGraph 表关系图
type RelationshipGraph struct {
	Tables        map[string]*TableNode `json:"tables"`
	Relationships []*core.Relationship  `json:"relationships"`
}

// TableNode 表节点
type TableNode struct {
	Table         *core.TableInfo      `json:"table"`
	IncomingRefs  []*core.Relationship `json:"incoming_refs"`
	OutgoingRefs  []*core.Relationship `json:"outgoing_refs"`
	RelatedTables map[string]bool      `json:"related_tables"`
}

// GetRelatedTables 获取相关表列表
func (n *TableNode) GetRelatedTables() []string {
	tables := make([]string, 0, len(n.RelatedTables))
	for table := range n.RelatedTables {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	return tables
}

// GetDirectRelationships 获取直接关系
func (n *TableNode) GetDirectRelationships() []*core.Relationship {
	relationships := make([]*core.Relationship, 0)
	relationships = append(relationships, n.IncomingRefs...)
	relationships = append(relationships, n.OutgoingRefs...)
	return relationships
}

// QueryOptimizationSuggestion 查询优化建议
type QueryOptimizationSuggestion struct {
	Type        string   `json:"type"` // join_order, index_usage, etc.
	Description string   `json:"description"`
	Tables      []string `json:"tables"`
	Columns     []string `json:"columns,omitempty"`
	Priority    int      `json:"priority"` // 1-10, 10 is highest
}

// GenerateQueryOptimizations 生成查询优化建议
func (g *RelationshipGraph) GenerateQueryOptimizations(involvedTables []string) []*QueryOptimizationSuggestion {
	var suggestions []*QueryOptimizationSuggestion

	if len(involvedTables) < 2 {
		return suggestions
	}

	// 分析连接路径
	joinPaths := g.findOptimalJoinPaths(involvedTables)
	for _, path := range joinPaths {
		suggestion := &QueryOptimizationSuggestion{
			Type:        "join_order",
			Description: fmt.Sprintf("建议的连接顺序: %s", strings.Join(path.Tables, " -> ")),
			Tables:      path.Tables,
			Priority:    path.Priority,
		}
		suggestions = append(suggestions, suggestion)
	}

	// 分析索引使用
	indexSuggestions := g.analyzeIndexUsage(involvedTables)
	suggestions = append(suggestions, indexSuggestions...)

	// 分析查询复杂度
	complexitySuggestions := g.analyzeQueryComplexity(involvedTables)
	suggestions = append(suggestions, complexitySuggestions...)

	return suggestions
}

// JoinPath 连接路径
type JoinPath struct {
	Tables   []string `json:"tables"`
	Cost     int      `json:"cost"`
	Priority int      `json:"priority"`
}

// findOptimalJoinPaths 查找最优连接路径
func (g *RelationshipGraph) findOptimalJoinPaths(tables []string) []*JoinPath {
	var paths []*JoinPath

	// 简单实现：基于表大小和关系强度排序
	for i, table1 := range tables {
		for j := i + 1; j < len(tables); j++ {
			table2 := tables[j]

			if g.hasDirectRelationship(table1, table2) {
				path := &JoinPath{
					Tables:   []string{table1, table2},
					Cost:     g.calculateJoinCost(table1, table2),
					Priority: 8,
				}
				paths = append(paths, path)
			}
		}
	}

	// 按成本排序
	sort.Slice(paths, func(i, j int) bool {
		return paths[i].Cost < paths[j].Cost
	})

	return paths
}

// hasDirectRelationship 检查两表是否有直接关系
func (g *RelationshipGraph) hasDirectRelationship(table1, table2 string) bool {
	node1, exists1 := g.Tables[table1]
	if !exists1 {
		return false
	}

	return node1.RelatedTables[table2]
}

// calculateJoinCost 计算连接成本
func (g *RelationshipGraph) calculateJoinCost(table1, table2 string) int {
	// 简单的成本计算：基于表大小
	cost := 1

	if node1, exists := g.Tables[table1]; exists {
		cost += int(node1.Table.RowCount / 1000) // 每1000行增加1点成本
	}

	if node2, exists := g.Tables[table2]; exists {
		cost += int(node2.Table.RowCount / 1000)
	}

	return cost
}

// analyzeIndexUsage 分析索引使用
func (g *RelationshipGraph) analyzeIndexUsage(tables []string) []*QueryOptimizationSuggestion {
	var suggestions []*QueryOptimizationSuggestion

	for _, tableName := range tables {
		if node, exists := g.Tables[tableName]; exists {
			// 检查外键列是否有索引
			for _, fk := range node.Table.ForeignKeys {
				hasIndex := false
				for _, index := range node.Table.Indexes {
					for _, col := range index.Columns {
						if col == fk.Column {
							hasIndex = true
							break
						}
					}
					if hasIndex {
						break
					}
				}

				if !hasIndex {
					suggestion := &QueryOptimizationSuggestion{
						Type:        "index_usage",
						Description: fmt.Sprintf("建议在表 %s 的列 %s 上创建索引以优化连接性能", tableName, fk.Column),
						Tables:      []string{tableName},
						Columns:     []string{fk.Column},
						Priority:    7,
					}
					suggestions = append(suggestions, suggestion)
				}
			}
		}
	}

	return suggestions
}

// analyzeQueryComplexity 分析查询复杂度
func (g *RelationshipGraph) analyzeQueryComplexity(tables []string) []*QueryOptimizationSuggestion {
	var suggestions []*QueryOptimizationSuggestion

	if len(tables) > 5 {
		suggestion := &QueryOptimizationSuggestion{
			Type:        "complexity",
			Description: fmt.Sprintf("查询涉及 %d 个表，建议考虑分解为多个简单查询或使用视图", len(tables)),
			Tables:      tables,
			Priority:    6,
		}
		suggestions = append(suggestions, suggestion)
	}

	// 检查是否有循环依赖
	if g.hasCyclicDependency(tables) {
		suggestion := &QueryOptimizationSuggestion{
			Type:        "cyclic_dependency",
			Description: "检测到表之间存在循环依赖，可能影响查询性能",
			Tables:      tables,
			Priority:    5,
		}
		suggestions = append(suggestions, suggestion)
	}

	return suggestions
}

// hasCyclicDependency 检查是否有循环依赖
func (g *RelationshipGraph) hasCyclicDependency(tables []string) bool {
	// 简单的循环检测实现
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for _, table := range tables {
		if !visited[table] {
			if g.dfsHasCycle(table, visited, recStack) {
				return true
			}
		}
	}

	return false
}

// dfsHasCycle DFS 循环检测
func (g *RelationshipGraph) dfsHasCycle(table string, visited, recStack map[string]bool) bool {
	visited[table] = true
	recStack[table] = true

	if node, exists := g.Tables[table]; exists {
		for _, rel := range node.OutgoingRefs {
			if !visited[rel.ToTable] {
				if g.dfsHasCycle(rel.ToTable, visited, recStack) {
					return true
				}
			} else if recStack[rel.ToTable] {
				return true
			}
		}
	}

	recStack[table] = false
	return false
}
