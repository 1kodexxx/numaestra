export interface ExampleSong {
  id: string
  title: string
  category: string
  description: string
  mood: string
  audioUrl: string
}

export const EXAMPLE_SONGS: ExampleSong[] = [
  {
    id: 'wedding-ballad',
    title: 'Свадебная баллада',
    category: 'Свадьба',
    description: 'Нежная баллада о любви двух людей, написанная специально для свадебного вечера. Лирика отражает историю пары.',
    mood: 'Романтика',
    audioUrl: '',
  },
  {
    id: 'birthday-jazz',
    title: 'Именинный джаз',
    category: 'День рождения',
    description: 'Джазовая композиция с тёплым поздравлением имениннику. Живой ритм и позитивный настрой.',
    mood: 'Радость',
    audioUrl: '',
  },
  {
    id: 'corporate-anthem',
    title: 'Корпоративный гимн',
    category: 'Корпоратив',
    description: 'Энергичный гимн компании с упоминанием достижений команды и ценностей бренда.',
    mood: 'Энергия',
    audioUrl: '',
  },
  {
    id: 'roast-comedy',
    title: 'Дружеский роуст',
    category: 'Роуст',
    description: 'Весёлая пародийная песня о «недостатках» друга, которая поднимает настроение всей компании.',
    mood: 'Юмор',
    audioUrl: '',
  },
  {
    id: 'anniversary-pop',
    title: 'Юбилейный поп',
    category: 'Юбилей',
    description: 'Лёгкая поп-песня с ретро-настроением. Тёплые слова об ушедших годах и взгляд в будущее.',
    mood: 'Ностальгия',
    audioUrl: '',
  },
]
