export const messages = {
  zh: {
    dashboard: '控制面板',
    inbounds: '入站管理',
    subscription: '订阅管理',
    tools: '工具箱',
    logs: '日志中心',
    settings: '系统设置',
    welcome: 'Welcome!',
    moduleReady: 'READY!',
    cpuStatus: 'CPU 状态',
    inboundTotal: '入站总数',
    inboundEnabled: '启用入站',
    trafficUsage: '流量使用',
    search: '搜索节点或日志...',
    logout: '退出登录',
    manage: '管理菜单',
    refresh: '刷新',
    darkLight: '深浅切换',
    language: '语言',
  },
  en: {
    dashboard: 'Dashboard',
    inbounds: 'Inbounds',
    subscription: 'Subscription',
    tools: 'Tools',
    logs: 'Logs',
    settings: 'Settings',
    welcome: 'Welcome back. System is running well.',
    moduleReady: 'Module is ready for direct operations.',
    cpuStatus: 'CPU Status',
    inboundTotal: 'Total Inbounds',
    inboundEnabled: 'Enabled Inbounds',
    trafficUsage: 'Traffic Usage',
    search: 'Search nodes or logs...',
    logout: 'Logout',
    manage: 'MANAGE',
    refresh: 'Refresh',
    darkLight: 'Theme',
    language: 'Language',
  },
}

export function useT(lang) {
  const current = messages[lang] || messages.zh
  return (key, fallback = '') => current[key] || fallback || key
}
