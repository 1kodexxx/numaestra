import { describe, it, expect } from 'vitest'
import { detectRiskyTerms } from './lyricsGuard'

describe('detectRiskyTerms', () => {
  it('ловит реального артиста в тексте', () => {
    expect(detectRiskyTerms('Пацаны на лимузине, как Ludacris')).toEqual(['ludacris'])
  })

  it('ловит бренд и артиста, без повторов', () => {
    const got = detectRiskyTerms('Еду в Wildberries, слушаю Моргенштерн и опять wildberries')
    expect(got).toContain('wildberries')
    expect(got).toContain('моргенштерн')
    expect(got.filter((t) => t === 'wildberries')).toHaveLength(1)
  })

  it('регистр не важен', () => {
    expect(detectRiskyTerms('LUDACRIS и EMINEM').sort()).toEqual(['eminem', 'ludacris'])
  })

  it('не ловит на подстроке внутри слова', () => {
    // "драконий" не должен ловить "drake"; "баста" отдельно — ловит, тут проверяем границу
    expect(detectRiskyTerms('драконий костёр')).toEqual([])
  })

  it('пустой/обычный текст — ничего не находит', () => {
    expect(detectRiskyTerms('')).toEqual([])
    expect(detectRiskyTerms('Песня про любовь для мамы на юбилей')).toEqual([])
  })

  it('матч по границе слова через пунктуацию', () => {
    expect(detectRiskyTerms('...как eminem!')).toEqual(['eminem'])
  })
})
