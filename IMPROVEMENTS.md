# Process Tracker 改进说明

## 📊 本次改进 (v0.3.9)

### 1. 智能单位显示

**问题**: 所有内存都显示为MB，需要数位数才能区分大小

**解决**: 自动转换为合适单位

```
之前: 15113.7MB
现在: 14.76GB

之前: 1414.5MB  
现在: 1.38GB

之前: 444.6MB
现在: 444.6MB
```

### 2. 短参数支持

所有统计命令现在支持简洁的短参数：

| 参数 | 功能 | 示例 |
|------|------|------|
| `-g` | 统计粒度 | `-g detailed` |
| `-s` | 排序 | `-s memory` |
| `-f` | 筛选 | `-f docker` |
| `-c` | 分类 | `-c development` |
| `-n` | 数量 | `-n 10` |

### 3. 增强功能

- ✅ 排序: 按cpu/memory/time/disk/network排序
- ✅ 筛选: 按进程名或分类筛选
- ✅ 汇总统计: 显示总体资源使用情况
- ✅ 内存占比: 显示每个进程的内存占比
- ✅ 对比功能: 对比不同时间段的数据
- ✅ 趋势分析: 查看多天的资源使用趋势
- ✅ CSV导出: 支持CSV格式导出

## 🚀 常用命令

```bash
# 内存Top 5
process-tracker today -s memory -n 5

# Docker容器监控
process-tracker today -f docker

# CPU分析
process-tracker today -s cpu -n 10 -g detailed

# 对比昨天
process-tracker compare today yesterday

# 导出CSV
process-tracker export --format csv
```

## 📝 记忆口诀

```
-g  粒度 (Granularity)
-s  排序 (Sort)
-f  筛选 (Filter)
-c  分类 (Category)
-n  数量 (Number)
```

---

**版本**: v0.3.9  
**更新日期**: 2024-10-15
