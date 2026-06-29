import { describe, expect, it } from 'vitest'
import { suggestEmailFix } from './emailHint'

describe('suggestEmailFix', () => {
  it('исправляет частые опечатки домена', () => {
    expect(suggestEmailFix('ivan@gmial.com')).toBe('ivan@gmail.com')
    expect(suggestEmailFix('ivan@gmail.con')).toBe('ivan@gmail.com')
    expect(suggestEmailFix('ivan@gmail.ru')).toBe('ivan@gmail.com')
    expect(suggestEmailFix('petr@yandex.ry')).toBe('petr@yandex.ru')
    expect(suggestEmailFix('petr@mail.ri')).toBe('petr@mail.ru')
  })

  it('ловит опечатки по расстоянию Левенштейна (нет в явной таблице)', () => {
    expect(suggestEmailFix('a@gmaol.com')).toBe('a@gmail.com')
    expect(suggestEmailFix('a@outlook.cm')).toBe('a@outlook.com')
  })

  it('молчит для корректных популярных доменов', () => {
    expect(suggestEmailFix('ivan@gmail.com')).toBeNull()
    expect(suggestEmailFix('ivan@yandex.ru')).toBeNull()
    expect(suggestEmailFix('ivan@mail.ru')).toBeNull()
    expect(suggestEmailFix('ivan@bk.ru')).toBeNull()
  })

  it('сохраняет локальную часть как есть (регистр, точки)', () => {
    expect(suggestEmailFix('Ivan.Petrov+tag@gmial.com')).toBe('Ivan.Petrov+tag@gmail.com')
  })

  it('не трогает незнакомые корпоративные домены', () => {
    expect(suggestEmailFix('user@numaestra.ru')).toBeNull()
    expect(suggestEmailFix('user@some-very-unique-corp.io')).toBeNull()
  })

  it('возвращает null на мусоре и пустых значениях', () => {
    expect(suggestEmailFix('')).toBeNull()
    expect(suggestEmailFix('просто строка')).toBeNull()
    expect(suggestEmailFix('@gmail.com')).toBeNull()
    expect(suggestEmailFix('ivan@')).toBeNull()
  })
})
