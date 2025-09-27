#!/bin/bash

# 分析昨天的进程使用数据

echo "=== 2025年9月26日 进程使用分析报告 ==="
echo ""

# 提取昨天数据的时间范围
START_TIME=1758854400  # 2025-09-26 00:00:00
END_TIME=1758940799    # 2025-09-26 23:59:59

# 提取昨天的数据
awk -F',' -v start="$START_TIME" -v end="$END_TIME" 'NR>1 && $1 >= start && $1 <= end {print}' /home/qy113/.process-tracker.log > yesterday_full.csv

# 统计CPU使用率最高的进程
echo "🔥 CPU使用率最高的进程："
echo ""
awk -F',' '{ 
    if ($2 != "0.00") {
        cpu_total[$2] += $2
        mem_total[$2] += $3
        count[$2]++
    }
} END {
    for (proc in cpu_total) {
        printf "%s,%.2f,%.2f,%d\n", proc, cpu_total[proc], mem_total[proc]/count[proc], count[proc]
    }
}' yesterday_full.csv | sort -t',' -k2 -nr | head -10

echo ""
echo "💾 内存使用最多的进程："
echo ""
awk -F',' '{ 
    if ($4 != "0.00") {
        mem_total[$2] += $4
        cpu_total[$2] += $3
        count[$2]++
    }
} END {
    for (proc in mem_total) {
        avg_mem = mem_total[proc] / count[proc]
        avg_cpu = cpu_total[proc] / count[proc]
        printf "%s,%.2f,%.2f,%d\n", proc, avg_mem, avg_cpu, count[proc]
    }
}' yesterday_full.csv | sort -t',' -k2 -nr | head -10

echo ""
echo "📊 最活跃的进程（出现次数最多）："
echo ""
awk -F',' '{count[$2]++} END {for (proc in count) printf "%s,%d\n", proc, count[proc]}' yesterday_full.csv | sort -t',' -k2 -nr | head -10

echo ""
echo "📈 数据样本总数："
wc -l yesterday_full.csv