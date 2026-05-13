import React from 'react'
import { Search, Sun, Moon } from 'lucide-react'

export default function Topbar({
  username,
  role = 'admin',
  onToggleTheme,
  theme,
  lang,
  onLangChange,
  t,
  searchValue = '',
  searchTarget = 'inbounds',
  onSearchChange = () => {},
  onSearchTargetChange = () => {},
  onSearchSubmit = () => {},
}) {
  return (
    <header className="topbar-new">
      <form className="search-box" onSubmit={onSearchSubmit} role="search">
        <Search size={16} aria-hidden="true" />
        <select
          value={searchTarget}
          onChange={(e) => onSearchTargetChange(e.target.value)}
          aria-label={t('search')}
        >
          <option value="inbounds">{t('inbounds')}</option>
          <option value="logs">{t('logs')}</option>
        </select>
        <input
          value={searchValue}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder={t('search')}
          aria-label={t('search')}
        />
      </form>

      <div className="topbar-actions">
        <select
          value={lang}
          onChange={(e) => onLangChange(e.target.value)}
          aria-label={t('language')}
        >
          <option value="zh">中文</option>
          <option value="en">English</option>
        </select>
        <button
          type="button"
          className="icon-btn"
          onClick={onToggleTheme}
          aria-label={t('darkLight')}
        >
          {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
        </button>
        <div className="avatar-chip" aria-label={`${username} (${role})`}>
          <div className="avatar" aria-hidden="true" />
          <div>
            <strong>{username || 'admin'}</strong>
            <span className="role-pill">{role}</span>
          </div>
        </div>
      </div>
    </header>
  )
}
