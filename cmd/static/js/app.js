// Process Tracker Dashboard

class Dashboard {
    constructor() {
        this.cpuChart = null;
        this.memoryChart = null;
        this.refreshInterval = 5000; // 5 seconds
        this.currentPeriod = 'today';
        this.currentSort = 'cpu';
        this.currentSearch = '';      // Search keyword
        this.currentCategory = '';    // Category filter
        this.allProcesses = [];       // All processes data
        
        this.init();
    }

    async init() {
        console.log('初始化Dashboard...');
        
        // Initialize charts
        this.initCharts();
        
        // Setup event listeners
        this.setupEventListeners();
        
        // Load initial data
        await this.loadStats();
        await this.loadProcesses();
        
        // Start auto-refresh - separate stats and processes
        setInterval(() => {
            this.loadStats();        // Refresh statistics
            this.loadProcesses();    // Refresh process list (preserves user sort/filter)
        }, this.refreshInterval);
    }

    setupEventListeners() {
        // Period selector
        document.getElementById('period-selector').addEventListener('change', (e) => {
            this.currentPeriod = e.target.value;
            this.loadStats();
        });

        // Sort buttons
        document.querySelectorAll('.sort-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                this.currentSort = e.target.dataset.sort;
                this.loadProcesses();
                
                // Update button styles
                document.querySelectorAll('.sort-btn').forEach(b => {
                    b.classList.remove('bg-blue-100', 'text-blue-600');
                });
                e.target.classList.add('bg-blue-100', 'text-blue-600');
            });
        });

        // Search box
        const searchBox = document.getElementById('process-search');
        if (searchBox) {
            searchBox.addEventListener('input', (e) => {
                this.currentSearch = e.target.value.toLowerCase();
                this.filterAndDisplayProcesses();
            });
        }

        // Filter buttons
        document.querySelectorAll('.filter-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                this.currentCategory = e.target.dataset.filter;
                this.filterAndDisplayProcesses();
                
                // Update button styles
                document.querySelectorAll('.filter-btn').forEach(b => {
                    b.classList.remove('bg-blue-100', 'text-blue-600');
                });
                e.target.classList.add('bg-blue-100', 'text-blue-600');
            });
        });
    }

    async loadStats() {
        try {
            const response = await fetch(`/api/stats/${this.currentPeriod}`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const data = await response.json();
            this.updateUI(data);
            
            // Update last update time
            document.getElementById('last-update').textContent = new Date().toLocaleTimeString('zh-CN');
        } catch (error) {
            console.error('加载统计数据失败:', error);
            this.showError('无法加载数据，请检查服务器连接');
        }
    }

    async loadProcesses() {
        try {
            const response = await fetch(`/api/processes?sort=${this.currentSort}`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }
            
            const data = await response.json();
            this.allProcesses = data.processes || [];
            this.filterAndDisplayProcesses();
        } catch (error) {
            console.error('加载进程列表失败:', error);
        }
    }

    filterAndDisplayProcesses() {
        let filtered = [...this.allProcesses];
        
        // Search filter
        if (this.currentSearch) {
            filtered = filtered.filter(p => 
                p.name.toLowerCase().includes(this.currentSearch) ||
                (p.command && p.command.toLowerCase().includes(this.currentSearch))
            );
        }
        
        // Category filter
        if (this.currentCategory) {
            if (this.currentCategory === 'docker') {
                filtered = filtered.filter(p => 
                    p.name.startsWith('docker:') || p.category === 'docker'
                );
            } else {
                filtered = filtered.filter(p => 
                    p.category === this.currentCategory
                );
            }
        }
        
        this.updateProcessTable(filtered);
    }

    updateUI(data) {
        // Update overview cards
        document.getElementById('active-count').textContent = data.active_count || 0;
        document.getElementById('total-count').textContent = data.process_count || 0;
        document.getElementById('avg-cpu').textContent = this.formatPercent(data.avg_cpu);
        document.getElementById('max-cpu').textContent = this.formatPercent(data.max_cpu);
        
        // Update memory with percentage
        const totalMemStr = `${this.formatMemory(data.total_memory)} (${(data.total_memory_percent || 0).toFixed(1)}%)`;
        const maxMemStr = `${this.formatMemory(data.max_memory)} (${(data.max_memory_percent || 0).toFixed(1)}%)`;
        document.getElementById('total-memory').textContent = totalMemStr;
        document.getElementById('max-memory').textContent = maxMemStr;
        
        // Update system information
        document.getElementById('total-cpu-cores').textContent = data.total_cpu_cores || '-';
        document.getElementById('system-total-memory').textContent = this.formatMemory(data.system_total_memory);
        
        // Update system status (now based on normalized CPU 0-100%)
        if (data.avg_cpu > 80) {
            document.getElementById('system-status').textContent = '高负载';
            document.getElementById('system-status').className = 'stat-value text-red-600';
        } else if (data.avg_cpu > 50) {
            document.getElementById('system-status').textContent = '中等';
            document.getElementById('system-status').className = 'stat-value text-yellow-600';
        } else {
            document.getElementById('system-status').textContent = '正常';
            document.getElementById('system-status').className = 'stat-value text-green-600';
        }
        
        // Update charts
        this.updateCharts(data.timeline);
        
        // Don't update process table here - it's handled by loadProcesses()
    }

    initCharts() {
        const chartOptions = {
            responsive: true,
            maintainAspectRatio: true,
            interaction: {
                mode: 'index',
                intersect: false,
            },
            plugins: {
                legend: {
                    display: false
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    grid: {
                        color: 'rgba(0, 0, 0, 0.05)'
                    }
                },
                x: {
                    grid: {
                        display: false
                    }
                }
            }
        };

        // CPU Chart
        const cpuCtx = document.getElementById('cpuChart').getContext('2d');
        this.cpuChart = new Chart(cpuCtx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: 'CPU使用率 (%)',
                    data: [],
                    borderColor: 'rgb(59, 130, 246)',
                    backgroundColor: 'rgba(59, 130, 246, 0.1)',
                    tension: 0.4,
                    fill: true
                }]
            },
            options: {
                ...chartOptions,
                scales: {
                    ...chartOptions.scales,
                    y: {
                        ...chartOptions.scales.y,
                        max: 100,
                        ticks: {
                            callback: function(value) {
                                return value + '%';
                            }
                        }
                    }
                },
                plugins: {
                    ...chartOptions.plugins,
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                return 'CPU: ' + context.parsed.y.toFixed(2) + '%';
                            }
                        }
                    }
                }
            }
        });

        // Memory Chart
        const memoryCtx = document.getElementById('memoryChart').getContext('2d');
        this.memoryChart = new Chart(memoryCtx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: '内存使用率 (%)',
                    data: [],
                    borderColor: 'rgb(16, 185, 129)',
                    backgroundColor: 'rgba(16, 185, 129, 0.1)',
                    tension: 0.4,
                    fill: true
                }]
            },
            options: {
                ...chartOptions,
                scales: {
                    ...chartOptions.scales,
                    y: {
                        ...chartOptions.scales.y,
                        max: 100,
                        ticks: {
                            callback: function(value) {
                                return value + '%';
                            }
                        }
                    }
                },
                plugins: {
                    ...chartOptions.plugins,
                    tooltip: {
                        callbacks: {
                            label: function(context) {
                                return '内存: ' + context.parsed.y.toFixed(1) + '%';
                            }
                        }
                    }
                }
            }
        });
    }

    updateCharts(timeline) {
        if (!timeline || timeline.length === 0) {
            return;
        }

        const labels = timeline.map(t => this.formatTimeLabel(t.time));
        const cpuData = timeline.map(t => (t.cpu_percent_normalized || 0).toFixed(2)); // Use normalized CPU percent
        const memoryData = timeline.map(t => (t.memory_percent || 0).toFixed(1)); // Use memory_percent instead of memory

        // Update CPU chart
        this.cpuChart.data.labels = labels;
        this.cpuChart.data.datasets[0].data = cpuData;
        this.cpuChart.update('none'); // Update without animation for smoother refresh

        // Update Memory chart
        this.memoryChart.data.labels = labels;
        this.memoryChart.data.datasets[0].data = memoryData;
        this.memoryChart.update('none');
    }

    updateProcessTable(processes) {
        const tbody = document.getElementById('process-table-body');
        
        if (!processes || processes.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" class="px-4 py-4 text-center text-gray-500">暂无数据</td></tr>';
            return;
        }

        tbody.innerHTML = processes.map(p => `
            <tr class="hover:bg-gray-50">
                <td class="px-4 py-3 text-sm text-gray-700">${p.pid}</td>
                <td class="px-4 py-3 text-sm font-medium text-gray-900">${this.escapeHtml(p.name)}</td>
                <td class="px-4 py-3 text-sm">
                    <div class="flex items-center">
                        <div class="w-16">${this.formatPercent(p.cpu_percent)}</div>
                        <div class="flex-1 ml-2">
                            <div class="w-full bg-gray-200 rounded-full h-2">
                                <div class="bg-blue-500 h-2 rounded-full" style="width: ${Math.min(p.cpu_percent, 100)}%"></div>
                            </div>
                        </div>
                    </div>
                </td>
                <td class="px-4 py-3 text-sm text-gray-700">${this.formatMemory(p.memory_mb, p.memory_percent)}</td>
                <td class="px-4 py-3 text-sm text-gray-700">${p.uptime || '-'}</td>
                <td class="px-4 py-3 text-sm">
                    <span class="inline-flex items-center">
                        <span class="status-${p.status}"></span>
                        <span class="ml-2 text-gray-600">${p.status === 'active' ? '活跃' : '空闲'}</span>
                    </span>
                </td>
            </tr>
        `).join('');
    }

    // Utility functions
    formatPercent(value) {
        if (value === undefined || value === null) return '-';
        return `${value.toFixed(1)}%`;
    }

    formatMemory(mb, percent) {
        if (mb === undefined || mb === null) return '-';
        
        // Format memory size
        let sizeStr;
        if (mb < 1024) {
            sizeStr = `${Math.round(mb)} MB`;
        } else {
            sizeStr = `${(mb / 1024).toFixed(1)} GB`;
        }
        
        // Add percentage if available
        if (percent !== undefined && percent !== null && percent > 0) {
            return `${percent.toFixed(1)}% (${sizeStr})`;
        }
        
        return sizeStr;
    }

    formatTimeLabel(time) {
        // Input format: "2025-01-15 14:00"
        // Output format: "14:00" or "01-15 14:00" depending on period
        const parts = time.split(' ');
        if (this.currentPeriod === 'today') {
            return parts[1]; // Just time
        }
        return `${parts[0].substring(5)} ${parts[1]}`; // MM-DD HH:MM
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    showError(message) {
        // Simple error display
        console.error(message);
        // Could implement a toast notification here
    }
}

// Initialize dashboard when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => new Dashboard());
} else {
    new Dashboard();
}
