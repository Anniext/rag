package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQueryOptimizer(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	// Test with nil config
	optimizer := NewQueryOptimizer(nil, logger, metrics)
	assert.NotNil(t, optimizer)
	assert.NotNil(t, optimizer.config)

	// Test with custom config
	config := &OptimizerConfig{
		SlowQueryThreshold: 2 * time.Second,
		EnableQueryPlan:    false,
	}
	optimizer = NewQueryOptimizer(config, logger, metrics)
	assert.NotNil(t, optimizer)
	assert.Equal(t, 2*time.Second, optimizer.config.SlowQueryThreshold)
	assert.False(t, optimizer.config.EnableQueryPlan)
}

func TestQueryOptimizer_AnalyzeQuery(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}
	optimizer := NewQueryOptimizer(nil, logger, metrics)
	ctx := context.Background()

	tests := []struct {
		name          string
		sql           string
		executionTime time.Duration
		wantType      string
		wantTables    []string
		wantHasJoin   bool
		wantHasLimit  bool
	}{
		{
			name:          "simple select",
			sql:           "SELECT id, name FROM users",
			executionTime: 100 * time.Millisecond,
			wantType:      "SELECT",
			wantTables:    []string{"users"},
			wantHasJoin:   false,
			wantHasLimit:  false,
		},
		{
			name:          "select with join",
			sql:           "SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id",
			executionTime: 500 * time.Millisecond,
			wantType:      "SELECT",
			wantTables:    []string{"users", "posts"},
			wantHasJoin:   true,
			wantHasLimit:  false,
		},
		{
			name:          "select with limit",
			sql:           "SELECT * FROM users LIMIT 10",
			executionTime: 50 * time.Millisecond,
			wantType:      "SELECT",
			wantTables:    []string{"users"},
			wantHasJoin:   false,
			wantHasLimit:  true,
		},
		{
			name:          "insert query",
			sql:           "INSERT INTO users (name, email) VALUES ('John', 'john@example.com')",
			executionTime: 10 * time.Millisecond,
			wantType:      "INSERT",
			wantTables:    []string{"users"},
			wantHasJoin:   false,
			wantHasLimit:  false,
		},
		{
			name:          "update query",
			sql:           "UPDATE users SET name = 'Jane' WHERE id = 1",
			executionTime: 20 * time.Millisecond,
			wantType:      "UPDATE",
			wantTables:    []string{"users"},
			wantHasJoin:   false,
			wantHasLimit:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis, err := optimizer.AnalyzeQuery(ctx, tt.sql, tt.executionTime)
			require.NoError(t, err)
			assert.NotNil(t, analysis)

			assert.Equal(t, tt.sql, analysis.SQL)
			assert.Equal(t, tt.wantType, analysis.QueryType)
			assert.Equal(t, tt.wantTables, analysis.Tables)
			assert.Equal(t, tt.wantHasJoin, analysis.HasJoin)
			assert.Equal(t, tt.wantHasLimit, analysis.HasLimit)
			assert.Equal(t, tt.executionTime, analysis.ExecutionTime)
			assert.NotNil(t, analysis.Issues)
			assert.NotNil(t, analysis.Suggestions)
			assert.True(t, analysis.PerformanceScore >= 0 && analysis.PerformanceScore <= 100)
		})
	}
}

func TestQueryOptimizer_DetectQueryType(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, &mockLogger{}, &mockMetrics{})

	tests := []struct {
		sql      string
		expected string
	}{
		{"SELECT * FROM users", "SELECT"},
		{"select id from posts", "SELECT"},
		{"INSERT INTO users VALUES (1, 'John')", "INSERT"},
		{"UPDATE users SET name = 'Jane'", "UPDATE"},
		{"DELETE FROM users WHERE id = 1", "DELETE"},
		{"CREATE TABLE test (id INT)", "CREATE"},
		{"ALTER TABLE users ADD COLUMN age INT", "ALTER"},
		{"DROP TABLE test", "DROP"},
		{"EXPLAIN SELECT * FROM users", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			result := optimizer.detectQueryType(tt.sql)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryOptimizer_ExtractTables(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, &mockLogger{}, &mockMetrics{})

	tests := []struct {
		sql      string
		expected []string
	}{
		{
			"SELECT * FROM users",
			[]string{"users"},
		},
		{
			"SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id",
			[]string{"users", "posts"},
		},
		{
			"INSERT INTO users (name) VALUES ('John')",
			[]string{"users"},
		},
		{
			"UPDATE users SET name = 'Jane' WHERE id = 1",
			[]string{"users"},
		},
		{
			"SELECT * FROM users u LEFT JOIN profiles p ON u.id = p.user_id RIGHT JOIN settings s ON u.id = s.user_id",
			[]string{"users", "profiles", "settings"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			result := optimizer.extractTables(tt.sql)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestQueryOptimizer_ExtractColumns(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, &mockLogger{}, &mockMetrics{})

	tests := []struct {
		sql      string
		expected []string
	}{
		{
			"SELECT id, name FROM users",
			[]string{"id", "name"},
		},
		{
			"SELECT u.id, u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id",
			[]string{"u.id", "u.name", "p.title"},
		},
		{
			"SELECT * FROM users",
			[]string{}, // SELECT * doesn't extract specific columns
		},
		{
			"SELECT COUNT(*) FROM users",
			[]string{"COUNT(*)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			result := optimizer.extractColumns(tt.sql)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestQueryOptimizer_QueryFeatureDetection(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, &mockLogger{}, &mockMetrics{})

	tests := []struct {
		name         string
		sql          string
		hasJoin      bool
		hasSubquery  bool
		hasAggregate bool
		hasOrderBy   bool
		hasGroupBy   bool
		hasLimit     bool
	}{
		{
			name:         "simple select",
			sql:          "SELECT id, name FROM users",
			hasJoin:      false,
			hasSubquery:  false,
			hasAggregate: false,
			hasOrderBy:   false,
			hasGroupBy:   false,
			hasLimit:     false,
		},
		{
			name:         "with join",
			sql:          "SELECT u.name FROM users u JOIN posts p ON u.id = p.user_id",
			hasJoin:      true,
			hasSubquery:  false,
			hasAggregate: false,
			hasOrderBy:   false,
			hasGroupBy:   false,
			hasLimit:     false,
		},
		{
			name:         "with subquery",
			sql:          "SELECT * FROM users WHERE id IN (SELECT user_id FROM posts)",
			hasJoin:      false,
			hasSubquery:  true,
			hasAggregate: false,
			hasOrderBy:   false,
			hasGroupBy:   false,
			hasLimit:     false,
		},
		{
			name:         "with aggregate",
			sql:          "SELECT COUNT(*) FROM users",
			hasJoin:      false,
			hasSubquery:  false,
			hasAggregate: true,
			hasOrderBy:   false,
			hasGroupBy:   false,
			hasLimit:     false,
		},
		{
			name:         "with order by",
			sql:          "SELECT * FROM users ORDER BY name",
			hasJoin:      false,
			hasSubquery:  false,
			hasAggregate: false,
			hasOrderBy:   true,
			hasGroupBy:   false,
			hasLimit:     false,
		},
		{
			name:         "with group by",
			sql:          "SELECT department, COUNT(*) FROM users GROUP BY department",
			hasJoin:      false,
			hasSubquery:  false,
			hasAggregate: true,
			hasOrderBy:   false,
			hasGroupBy:   true,
			hasLimit:     false,
		},
		{
			name:         "with limit",
			sql:          "SELECT * FROM users LIMIT 10",
			hasJoin:      false,
			hasSubquery:  false,
			hasAggregate: false,
			hasOrderBy:   false,
			hasGroupBy:   false,
			hasLimit:     true,
		},
		{
			name:         "complex query",
			sql:          "SELECT u.name, COUNT(p.id) FROM users u LEFT JOIN posts p ON u.id = p.user_id WHERE u.id IN (SELECT user_id FROM active_users) GROUP BY u.id ORDER BY COUNT(p.id) DESC LIMIT 10",
			hasJoin:      true,
			hasSubquery:  true,
			hasAggregate: true,
			hasOrderBy:   true,
			hasGroupBy:   true,
			hasLimit:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.hasJoin, optimizer.hasJoin(tt.sql))
			assert.Equal(t, tt.hasSubquery, optimizer.hasSubquery(tt.sql))
			assert.Equal(t, tt.hasAggregate, optimizer.hasAggregate(tt.sql))
			assert.Equal(t, tt.hasOrderBy, optimizer.hasOrderBy(tt.sql))
			assert.Equal(t, tt.hasGroupBy, optimizer.hasGroupBy(tt.sql))
			assert.Equal(t, tt.hasLimit, optimizer.hasLimit(tt.sql))
		})
	}
}

func TestQueryOptimizer_CalculateComplexity(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, &mockLogger{}, &mockMetrics{})

	tests := []struct {
		name      string
		analysis  *QueryAnalysis
		wantLevel string
		wantScore float64
	}{
		{
			name: "simple query",
			analysis: &QueryAnalysis{
				Tables:       []string{"users"},
				HasJoin:      false,
				HasSubquery:  false,
				HasAggregate: false,
				HasGroupBy:   false,
				HasOrderBy:   false,
			},
			wantLevel: "LOW",
			wantScore: 1.0,
		},
		{
			name: "medium complexity",
			analysis: &QueryAnalysis{
				Tables:       []string{"users", "posts"},
				HasJoin:      true,
				HasSubquery:  false,
				HasAggregate: true,
				HasGroupBy:   true,
				HasOrderBy:   true,
			},
			wantLevel: "HIGH",
			wantScore: 7.0, // 2 tables (2.0) + 1 join (2.0) + aggregate (1.5) + group by (1.0) + order by (0.5)
		},
		{
			name: "high complexity",
			analysis: &QueryAnalysis{
				Tables:       []string{"users", "posts", "comments"},
				HasJoin:      true,
				HasSubquery:  true,
				HasAggregate: true,
				HasGroupBy:   true,
				HasOrderBy:   true,
			},
			wantLevel: "VERY_HIGH",
			wantScore: 13.0, // 3 tables (3.0) + 2 joins (4.0) + subquery (3.0) + aggregate (1.5) + group by (1.0) + order by (0.5) + subquery count (1.0)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := optimizer.calculateComplexity(tt.analysis)
			assert.Equal(t, tt.wantLevel, result.Level)
			assert.Equal(t, tt.wantScore, result.Score)
			assert.Equal(t, len(tt.analysis.Tables), result.TableCount)
		})
	}
}

func TestQueryOptimizer_DetectIssues(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, &mockLogger{}, &mockMetrics{})

	tests := []struct {
		name       string
		analysis   *QueryAnalysis
		wantIssues []string
	}{
		{
			name: "select all issue",
			analysis: &QueryAnalysis{
				SQL:        "SELECT * FROM users",
				QueryType:  "SELECT",
				HasLimit:   false,
				Complexity: QueryComplexity{Score: 1.0},
			},
			wantIssues: []string{"SELECT_ALL", "MISSING_LIMIT"},
		},
		{
			name: "high complexity issue",
			analysis: &QueryAnalysis{
				SQL:        "SELECT u.name FROM users u JOIN posts p ON u.id = p.user_id JOIN comments c ON p.id = c.post_id",
				QueryType:  "SELECT",
				HasLimit:   true,
				Tables:     []string{"users", "posts", "comments"},
				HasJoin:    true,
				Complexity: QueryComplexity{Score: 15.0},
			},
			wantIssues: []string{"HIGH_COMPLEXITY", "POTENTIAL_CARTESIAN"},
		},
		{
			name: "no issues",
			analysis: &QueryAnalysis{
				SQL:        "SELECT id, name FROM users LIMIT 10",
				QueryType:  "SELECT",
				HasLimit:   true,
				Tables:     []string{"users"},
				HasJoin:    false,
				Complexity: QueryComplexity{Score: 1.0},
			},
			wantIssues: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := optimizer.detectIssues(tt.analysis)

			issueTypes := make([]string, len(issues))
			for i, issue := range issues {
				issueTypes[i] = issue.Type
			}

			assert.ElementsMatch(t, tt.wantIssues, issueTypes)
		})
	}
}

func TestQueryOptimizer_CheckSlowQuery(t *testing.T) {
	config := &OptimizerConfig{
		SlowQueryThreshold: time.Second,
	}
	optimizer := NewQueryOptimizer(config, &mockLogger{}, &mockMetrics{})
	ctx := context.Background()

	// Test non-slow query
	alert := optimizer.CheckSlowQuery(ctx, "SELECT * FROM users", 500*time.Millisecond, "user1", "session1")
	assert.Nil(t, alert)

	// Test slow query
	alert = optimizer.CheckSlowQuery(ctx, "SELECT * FROM users", 2*time.Second, "user1", "session1")
	assert.NotNil(t, alert)
	assert.Equal(t, "SELECT * FROM users", alert.SQL)
	assert.Equal(t, 2*time.Second, alert.ExecutionTime)
	assert.Equal(t, "user1", alert.UserID)
	assert.Equal(t, "session1", alert.SessionID)
	assert.NotEmpty(t, alert.ID)
	assert.NotNil(t, alert.Analysis)
	assert.Equal(t, "LOW", alert.Severity)
}

func TestQueryOptimizer_DetermineSeverity(t *testing.T) {
	config := &OptimizerConfig{
		SlowQueryThreshold: time.Second,
	}
	optimizer := NewQueryOptimizer(config, &mockLogger{}, &mockMetrics{})

	tests := []struct {
		executionTime time.Duration
		expected      string
	}{
		{2 * time.Second, "LOW"},
		{3 * time.Second, "MEDIUM"},
		{6 * time.Second, "HIGH"},
		{10 * time.Second, "HIGH"},
		{11 * time.Second, "CRITICAL"},
		{20 * time.Second, "CRITICAL"},
	}

	for _, tt := range tests {
		t.Run(tt.executionTime.String(), func(t *testing.T) {
			result := optimizer.determineSeverity(tt.executionTime)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryOptimizer_OptimizeQuery(t *testing.T) {
	config := &OptimizerConfig{
		EnableQueryRewrite: true,
		MaxQueryLength:     10000,
	}
	optimizer := NewQueryOptimizer(config, &mockLogger{}, &mockMetrics{})
	ctx := context.Background()

	tests := []struct {
		name            string
		sql             string
		expectOptimized bool
	}{
		{
			name:            "query without limit",
			sql:             "SELECT * FROM users",
			expectOptimized: true,
		},
		{
			name:            "query with limit",
			sql:             "SELECT * FROM users LIMIT 10",
			expectOptimized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optimizedSQL, suggestions, err := optimizer.OptimizeQuery(ctx, tt.sql)
			require.NoError(t, err)

			if tt.expectOptimized {
				assert.NotEqual(t, tt.sql, optimizedSQL)
				assert.NotEmpty(t, suggestions)
			} else {
				assert.Equal(t, tt.sql, optimizedSQL)
			}
		})
	}
}

func TestQueryOptimizer_CalculatePerformanceScore(t *testing.T) {
	optimizer := NewQueryOptimizer(nil, &mockLogger{}, &mockMetrics{})

	tests := []struct {
		name     string
		analysis *QueryAnalysis
		wantMin  float64
		wantMax  float64
	}{
		{
			name: "perfect query",
			analysis: &QueryAnalysis{
				Complexity: QueryComplexity{Score: 0.0},
				Issues:     []QueryIssue{},
			},
			wantMin: 100.0,
			wantMax: 100.0,
		},
		{
			name: "query with issues",
			analysis: &QueryAnalysis{
				Complexity: QueryComplexity{Score: 2.0},
				Issues: []QueryIssue{
					{Severity: "HIGH"},
					{Severity: "MEDIUM"},
				},
			},
			wantMin: 60.0, // 100 - (2.0 * 5) - 20 - 10
			wantMax: 70.0,
		},
		{
			name: "very bad query",
			analysis: &QueryAnalysis{
				Complexity: QueryComplexity{Score: 20.0},
				Issues: []QueryIssue{
					{Severity: "HIGH"},
					{Severity: "HIGH"},
					{Severity: "HIGH"},
				},
			},
			wantMin: 0.0, // Should be clamped to 0
			wantMax: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := optimizer.calculatePerformanceScore(tt.analysis)
			assert.True(t, score >= tt.wantMin && score <= tt.wantMax,
				"Score %f should be between %f and %f", score, tt.wantMin, tt.wantMax)
			assert.True(t, score >= 0.0 && score <= 100.0,
				"Score should be between 0 and 100")
		})
	}
}
