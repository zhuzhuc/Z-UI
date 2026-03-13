import React from 'react'
import { Search, Sun, Moon } from 'lucide-react'

export default function Topbar({ username, onToggleTheme, theme, lang, onLangChange, t }) {
  return (
    <header className="topbar-new">
      <div className="search-box">
        <Search size={16} />
        <input placeholder={t('search', '搜索节点或日志...')} />
      </div>

      <div className="topbar-actions">
        <select value={lang} onChange={(e) => onLangChange(e.target.value)}>
          <option value="zh">中文</option>
          <option value="en">English</option>
        </select>
        <button type="button" className="icon-btn" onClick={onToggleTheme}>
          {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
        </button>
        <div className="avatar-chip">
          <div className="avatar" />
          <strong>{username || 'admin'}</strong>
        </div>
      </div>
    </header>
  )
}
