package session

import (
	"fmt"
)

// ExampleMemoryManager 展示内存管理器的基本功能
func ExampleMemoryManager() {
	// 演示相似度计算
	memoryManager := &MemoryManager{
		maxHistorySize:      100,
		contextWindow:       3600000000000, // 1 hour in nanoseconds
		similarityThreshold: 0.5,
	}

	// 测试相似度计算
	similarity1 := memoryManager.calculateSimilarity("select name from users", "select email from users")
	similarity2 := memoryManager.calculateSimilarity("select name from users", "insert into products")

	fmt.Printf("相似查询的相似度: %.2f\n", similarity1)
	fmt.Printf("不同查询的相似度: %.2f\n", similarity2)

	// 测试查询分类
	queryType1 := classifyQuery("SELECT * FROM users WHERE age > 18")
	queryType2 := classifyQuery("INSERT INTO users (name, email) VALUES ('John', 'john@example.com')")
	queryType3 := classifyQuery("UPDATE users SET status = 'active' WHERE id = 1")

	fmt.Printf("SELECT 查询类型: %s\n", queryType1)
	fmt.Printf("INSERT 查询类型: %s\n", queryType2)
	fmt.Printf("UPDATE 查询类型: %s\n", queryType3)

	// 测试时间段分类
	timeSlot1 := getTimeSlot(9)  // 上午
	timeSlot2 := getTimeSlot(15) // 下午
	timeSlot3 := getTimeSlot(21) // 晚上

	fmt.Printf("9点时间段: %s\n", timeSlot1)
	fmt.Printf("15点时间段: %s\n", timeSlot2)
	fmt.Printf("21点时间段: %s\n", timeSlot3)

	// Output:
	// 相似查询的相似度: 0.60
	// 不同查询的相似度: 0.00
	// SELECT 查询类型: 查询
	// INSERT 查询类型: 插入
	// UPDATE 查询类型: 更新
	// 9点时间段: 上午
	// 15点时间段: 下午
	// 21点时间段: 晚上
}
