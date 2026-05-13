import { describe, it, expect } from 'vitest'
import { messages, useT } from '../i18n'

describe('i18n messages', () => {
  it('has zh and en locales', () => {
    expect(messages.zh).toBeDefined()
    expect(messages.en).toBeDefined()
  })

  it('zh and en have the same keys', () => {
    const zhKeys = Object.keys(messages.zh).sort()
    const enKeys = Object.keys(messages.en).sort()
    expect(zhKeys).toEqual(enKeys)
  })

  it('all values are non-empty strings', () => {
    for (const [lang, dict] of Object.entries(messages)) {
      for (const [key, val] of Object.entries(dict)) {
        expect(typeof val, `${lang}.${key} should be string`).toBe('string')
        expect(val.length, `${lang}.${key} should be non-empty`).toBeGreaterThan(0)
      }
    }
  })

  it('has at least 80 translation keys', () => {
    expect(Object.keys(messages.zh).length).toBeGreaterThanOrEqual(80)
  })
})

describe('useT', () => {
  it('returns zh translations by default', () => {
    const t = useT('zh')
    expect(t('dashboard')).toBe('控制面板')
    expect(t('save')).toBe('保存')
  })

  it('returns en translations', () => {
    const t = useT('en')
    expect(t('dashboard')).toBe('Dashboard')
    expect(t('save')).toBe('Save')
  })

  it('falls back to zh for unknown lang', () => {
    const t = useT('fr')
    expect(t('dashboard')).toBe('控制面板')
  })

  it('returns fallback for missing key', () => {
    const t = useT('en')
    expect(t('nonexistent_key', 'fallback')).toBe('fallback')
  })

  it('returns key when no fallback and key missing', () => {
    const t = useT('en')
    expect(t('nonexistent_key')).toBe('nonexistent_key')
  })
})
