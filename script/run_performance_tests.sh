#!/bin/bash

# RAG 性能测试运行脚本
# 使用方法: ./run_performance_tests.sh [test_type] [options]

set -e

# 默认配置
DEFAULT_DURATION="1m"
DEFAULT_CONCURRENCY="10"
DEFAULT_MEMORY_SESSIONS="100"
DEFAULT_QUERIES_PER_SESSION="5"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印帮助信息
print_help() {
    echo "RAG 性能测试运行脚本"
    echo ""
    echo "使用方法:"
    echo "  $0 [test_type] [options]"
    echo ""
    echo "测试类型:"
    echo "  concurrent    - 并发查询压力测试"
    echo "  memory        - 内存使用压力测试"
    echo "  stability     - 长时间稳定性测试"
    echo "  all           - 运行所有测试 (默认)"
    echo ""
    echo "选项:"
    echo "  -d, --duration DURATION        测试持续时间 (默认: ${DEFAULT_DURATION})"
    echo "  -c, --concurrency CONCURRENCY  最大并发数 (默认: ${DEFAULT_CONCURRENCY})"
    echo "  -s, --sessions SESSIONS         内存测试会话数 (默认: ${DEFAULT_MEMORY_SESSIONS})"
    echo "  -q, --queries QUERIES           每会话查询数 (默认: ${DEFAULT_QUERIES_PER_SESSION})"
    echo "  -v, --verbose                   详细输出"
    echo "  -h, --help                      显示此帮助信息"
    echo ""
    echo "环境变量:"
    echo "  STRESS_DB_DSN                   数据库连接字符串"
    echo "  STRESS_ENABLE_PROFILING         启用性能分析 (true/false)"
    echo ""
    echo "示例:"
    echo "  $0 concurrent -d 30s -c 5       # 运行30秒并发测试，5个并发"
    echo "  $0 memory -s 50 -q 3             # 运行内存测试，50个会话，每会话3个查询"
    echo "  $0 stability -d 5m               # 运行5分钟稳定性测试"
    echo "  $0 all -v                        # 运行所有测试，详细输出"
}

# 打印彩色消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 Go 环境
check_go_env() {
    if ! command -v go &> /dev/null; then
        print_error "Go 未安装或不在 PATH 中"
        exit 1
    fi
    
    print_info "Go 版本: $(go version)"
}

# 运行并发查询测试
run_concurrent_test() {
    print_info "开始运行并发查询压力测试..."
    print_info "配置: 持续时间=${DURATION}, 并发数=${CONCURRENCY}"
    
    STRESS_TEST=true \
    STRESS_TEST_DURATION="${DURATION}" \
    STRESS_MAX_CONCURRENCY="${CONCURRENCY}" \
    go test -v -run  TestStressConcurrentQueries ../test -timeout $(($(echo ${DURATION} | sed 's/[^0-9]*//g') * 3))s
    
    if [ $? -eq 0 ]; then
        print_success "并发查询压力测试完成"
    else
        print_error "并发查询压力测试失败"
        return 1
    fi
}

# 运行内存压力测试
run_memory_test() {
    print_info "开始运行内存使用压力测试..."
    print_info "配置: 会话数=${MEMORY_SESSIONS}, 每会话查询数=${QUERIES_PER_SESSION}"
    
    STRESS_TEST=true \
    STRESS_MEMORY_SESSIONS="${MEMORY_SESSIONS}" \
    STRESS_QUERIES_PER_SESSION="${QUERIES_PER_SESSION}" \
    go test -v -run TestStressMemoryUsage ../test -timeout 5m
    
    if [ $? -eq 0 ]; then
        print_success "内存使用压力测试完成"
    else
        print_error "内存使用压力测试失败"
        return 1
    fi
}

# 运行稳定性测试
run_stability_test() {
    print_info "开始运行长时间稳定性测试..."
    print_info "配置: 持续时间=${DURATION}, 并发数=${CONCURRENCY}"
    
    STRESS_TEST=true \
    STRESS_TEST_DURATION="${DURATION}" \
    STRESS_MAX_CONCURRENCY="${CONCURRENCY}" \
    go test -v -run TestStressLongRunningStability ../test -timeout $(($(echo ${DURATION} | sed 's/[^0-9]*//g') * 2))s
    
    if [ $? -eq 0 ]; then
        print_success "长时间稳定性测试完成"
    else
        print_error "长时间稳定性测试失败"
        return 1
    fi
}

# 运行所有测试
run_all_tests() {
    print_info "开始运行所有性能测试..."
    
    local failed_tests=()
    
    # 运行并发测试
    if ! run_concurrent_test; then
        failed_tests+=("concurrent")
    fi
    
    # 运行内存测试
    if ! run_memory_test; then
        failed_tests+=("memory")
    fi
    
    # 运行稳定性测试
    if ! run_stability_test; then
        failed_tests+=("stability")
    fi
    
    # 汇总结果
    if [ ${#failed_tests[@]} -eq 0 ]; then
        print_success "所有性能测试都已通过！"
    else
        print_error "以下测试失败: ${failed_tests[*]}"
        return 1
    fi
}

# 解析命令行参数
TEST_TYPE="all"
DURATION="${DEFAULT_DURATION}"
CONCURRENCY="${DEFAULT_CONCURRENCY}"
MEMORY_SESSIONS="${DEFAULT_MEMORY_SESSIONS}"
QUERIES_PER_SESSION="${DEFAULT_QUERIES_PER_SESSION}"
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        concurrent|memory|stability|all)
            TEST_TYPE="$1"
            shift
            ;;
        -d|--duration)
            DURATION="$2"
            shift 2
            ;;
        -c|--concurrency)
            CONCURRENCY="$2"
            shift 2
            ;;
        -s|--sessions)
            MEMORY_SESSIONS="$2"
            shift 2
            ;;
        -q|--queries)
            QUERIES_PER_SESSION="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            print_help
            exit 0
            ;;
        *)
            print_error "未知选项: $1"
            print_help
            exit 1
            ;;
    esac
done

# 主程序
main() {
    print_info "RAG 性能测试开始"
    print_info "测试类型: ${TEST_TYPE}"
    
    # 检查环境
    check_go_env
    
    # 设置详细输出
    if [ "$VERBOSE" = true ]; then
        set -x
    fi
    
    # 记录开始时间
    START_TIME=$(date +%s)
    
    # 根据测试类型运行相应测试
    case $TEST_TYPE in
        concurrent)
            run_concurrent_test
            ;;
        memory)
            run_memory_test
            ;;
        stability)
            run_stability_test
            ;;
        all)
            run_all_tests
            ;;
        *)
            print_error "无效的测试类型: $TEST_TYPE"
            exit 1
            ;;
    esac
    
    # 计算总耗时
    END_TIME=$(date +%s)
    TOTAL_TIME=$((END_TIME - START_TIME))
    
    print_success "性能测试完成，总耗时: ${TOTAL_TIME}秒"
}

# 运行主程序
main "$@"