#!/bin/bash

echo "📋 2025年9月26日 详细进程使用分析报告"
echo "=========================================="
echo ""

# 总体统计
TOTAL_SAMPLES=$(wc -l < yesterday_full.csv)
ACTIVE_PROCESSES=$(awk -F',' '{print $2}' yesterday_full.csv | sort -u | wc -l)

echo "📊 总体统计："
echo "   • 数据样本总数: $TOTAL_SAMPLES 条"
echo "   • 不同进程数: $ACTIVE_PROCESSES 个"
echo "   • 监控时长: 24小时"
echo ""

echo "🔥 CPU使用率排行榜 TOP 10："
echo "   排名  进程名                    总CPU%  平均内存MB"
echo "   ----  ----------------------  --------  ----------"
awk -F',' '{cpu[$2] += $3; mem[$2] += $4; count[$2]++} 
END {
    for (proc in cpu) {
        if (count[proc] > 100) {  # 过滤掉出现次数太少的进程
            avg_mem = mem[proc] / count[proc]
            printf "%-30s %.2f %.2f\n", proc, cpu[proc], avg_mem
        }
    }
}' yesterday_full.csv | sort -k2 -nr | head -10 | nl | awk '{printf "   %2d    %-25s %8.2f  %10.2f\n", $1, $2, $3, $4}'

echo ""
echo "💾 内存使用排行榜 TOP 10："
echo "   排名  进程名                    平均内存MB  总CPU%"
echo "   ----  ----------------------  -----------  ------"
awk -F',' '{mem[$2] += $4; cpu[$2] += $3; count[$2]++} 
END {
    for (proc in mem) {
        if (count[proc] > 100) {
            avg_mem = mem[proc] / count[proc]
            printf "%-30s %.2f %.2f\n", proc, avg_mem, cpu[proc]
        }
    }
}' yesterday_full.csv | sort -k2 -nr | head -10 | nl | awk '{printf "   %2d    %-25s %12.2f  %6.2f\n", $1, $2, $3, $4}'

echo ""
echo "⏱️ 最持久运行的进程（出现次数最多）："
echo "   排名  进程名                    出现次数"
echo "   ----  ----------------------  --------"
awk -F',' '{count[$2]++} END {for (proc in count) printf "%-30s %d\n", proc, count[proc]}' yesterday_full.csv | sort -k2 -nr | head -10 | nl | awk '{printf "   %2d    %-25s %8d\n", $1, $2, $3}'

echo ""
echo "🎯 主要发现："
echo ""

# 分析主要服务
echo "🖥️ 系统服务："
NGINX_COUNT=$(awk -F',' '$2=="nginx"' yesterday_full.csv | wc -l)
POSTGRES_COUNT=$(awk -F',' '$2=="postgres"' yesterday_full.csv | wc -l)
DOCKER_COUNT=$(awk -F',' '$2=="docker-proxy"' yesterday_full.csv | wc -l)
echo "   • nginx: $NGINX_COUNT 次监控记录"
echo "   • postgres: $POSTGRES_COUNT 次监控记录" 
echo "   • docker-proxy: $DOCKER_COUNT 次监控记录"

echo ""
echo "💻 开发环境："
PYTHON_COUNT=$(awk -F',' '$2=="python"' yesterday_full.csv | wc -l)
NODE_COUNT=$(awk -F',' '$2=="node"' yesterday_full.csv | wc -l)
UV_COUNT=$(awk -F',' '$2=="uv"' yesterday_full.csv | wc -l)
ZSH_COUNT=$(awk -F',' '$2=="zsh"' yesterday_full.csv | wc -l)
echo "   • python: $PYTHON_COUNT 次监控记录"
echo "   • node: $NODE_COUNT 次监控记录"
echo "   • uv: $UV_COUNT 次监控记录"
echo "   • zsh: $ZSH_COUNT 次监控记录"

echo ""
echo "🌐 网络服务："
CLOUD_COUNT=$(awk -F',' '$2=="cloudflared"' yesterday_full.csv | wc -l)
VALKEY_COUNT=$(awk -F',' '$2=="valkey-server"' yesterday_full.csv | wc -l)
MYSQL_COUNT=$(awk -F',' '$2=="mysqld"' yesterday_full.csv | wc -l)
echo "   • cloudflared: $CLOUD_COUNT 次监控记录"
echo "   • valkey-server: $VALKEY_COUNT 次监控记录"
echo "   • mysqld: $MYSQL_COUNT 次监控记录"

echo ""
echo "📈 使用率趋势分析："
echo "   • CPU使用率最高的进程是 1panel-core 和 1panel-agent"
echo "   • 内存使用最多的是 remote-dev-server (约4.6GB)"
echo "   • nginx 是最活跃的进程，监控记录超过60万次"
echo "   • postgres 数据库服务持续运行，记录超过21万次"

echo ""
echo "⚡ 性能特征："
echo "   • 系统负载较低，大部分进程CPU使用率接近0%"
echo "   • 内存使用主要集中在开发工具和数据库服务"
echo "   • 网络服务(cloudflared)和容器服务(docker)持续稳定运行"