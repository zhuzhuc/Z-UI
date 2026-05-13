import { describe, it, expect } from 'vitest'
import { normalizePanelSettings, defaultPanelSettings } from '../panelSettings'

describe('normalizePanelSettings', () => {
  it('returns defaults for empty input', () => {
    const result = normalizePanelSettings({})
    expect(result.title).toBe(defaultPanelSettings.title)
    expect(result.language).toBe('zh')
    expect(result.theme).toBe('dark')
    expect(result.refreshIntervalSec).toBe(defaultPanelSettings.refreshIntervalSec)
    expect(result.requireLogin).toBe(true)
    expect(result.allowRegister).toBe(false)
  })

  it('normalizes language to en', () => {
    expect(normalizePanelSettings({ language: 'en' }).language).toBe('en')
    expect(normalizePanelSettings({ language: 'en-US' }).language).toBe('en')
    expect(normalizePanelSettings({ language: 'EN' }).language).toBe('en')
  })

  it('normalizes language to zh for non-en', () => {
    expect(normalizePanelSettings({ language: 'zh' }).language).toBe('zh')
    expect(normalizePanelSettings({ language: 'zh-CN' }).language).toBe('zh')
    expect(normalizePanelSettings({ language: 'fr' }).language).toBe('zh')
  })

  it('normalizes theme', () => {
    expect(normalizePanelSettings({ theme: 'light' }).theme).toBe('light')
    expect(normalizePanelSettings({ theme: 'dark' }).theme).toBe('dark')
    expect(normalizePanelSettings({ theme: 'invalid' }).theme).toBe('dark')
    expect(normalizePanelSettings({ theme: 'LIGHT' }).theme).toBe('light')
  })

  it('enforces minimum refresh interval', () => {
    expect(normalizePanelSettings({ refreshIntervalSec: 0 }).refreshIntervalSec).toBe(1)
    expect(normalizePanelSettings({ refreshIntervalSec: -5 }).refreshIntervalSec).toBe(1)
    expect(normalizePanelSettings({ refreshIntervalSec: 30 }).refreshIntervalSec).toBe(30)
  })

  it('trims title', () => {
    expect(normalizePanelSettings({ title: '  My Panel  ' }).title).toBe('My Panel')
    expect(normalizePanelSettings({ title: '' }).title).toBe(defaultPanelSettings.title)
  })

  it('trims publicBaseUrl', () => {
    expect(normalizePanelSettings({ publicBaseUrl: '  https://example.com  ' }).publicBaseUrl).toBe('https://example.com')
    expect(normalizePanelSettings({ publicBaseUrl: '' }).publicBaseUrl).toBe('')
  })

  it('normalizes boolean fields', () => {
    expect(normalizePanelSettings({ requireLogin: false }).requireLogin).toBe(false)
    expect(normalizePanelSettings({ requireLogin: undefined }).requireLogin).toBe(true)
    expect(normalizePanelSettings({ allowRegister: true }).allowRegister).toBe(true)
    expect(normalizePanelSettings({ allowRegister: 'truthy' }).allowRegister).toBe(true)
  })

  it('preserves unknown fields via spread', () => {
    const result = normalizePanelSettings({ customField: 'test' })
    expect(result.customField).toBe('test')
  })
})
