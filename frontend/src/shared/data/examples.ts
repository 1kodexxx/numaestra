export interface ExampleSong {
  id: string;
  title: string;
  category: string;
  description: string;
  mood: string;
  audioUrl: string;
  coverUrl: string;
}

export const EXAMPLE_SONGS: ExampleSong[] = [
  {
    id: "coral-wedding",
    title: "Коралловая свадьба",
    category: "Годовщина",
    description:
      "Тёплая песня к юбилею свадьбы — о прожитых вместе годах, верности и любви, которая только крепнет со временем.",
    mood: "Торжество",
    audioUrl: "/examples/wedding.mp3",
    coverUrl: "/examples/wedding.webp",
  },
  {
    id: "volodya-praskovya",
    title: "Володя и Прасковья",
    category: "Любовь",
    description:
      "Лиричная история пары — Володи и Прасковьи. Имена и судьбы героев вплетены прямо в текст песни.",
    mood: "Романтика",
    audioUrl: "/examples/volodya-praskovya.mp3",
    coverUrl: "/examples/volodya-praskovya.webp",
  },
  {
    id: "family-circle",
    title: "Звенит семейный круг",
    category: "Семья",
    description:
      "Душевный гимн большой семьи — о тепле родного дома, поколениях и связи, что объединяет всех за одним столом.",
    mood: "Тепло",
    audioUrl: "/examples/family-circle.mp3",
    coverUrl: "/examples/family-circle.webp",
  },
  {
    id: "jubilee-smirnov",
    title: "На юбилей Жени Смирнову",
    category: "Юбилей",
    description:
      "Персональное поздравление к юбилею — с именем виновника торжества и тёплыми словами от близких.",
    mood: "Праздник",
    audioUrl: "/examples/jubilee-smirnov.mp3",
    coverUrl: "/examples/jubilee-smirnov.webp",
  },
  {
    id: "jubilee-zhenya",
    title: "С юбилеем, Женя",
    category: "Юбилей",
    description:
      "Праздничная песня-поздравление: искренние пожелания и добрый юмор для дорогого человека в его особенный день.",
    mood: "Праздник",
    audioUrl: "/examples/jubilee-zhenya.mp3",
    coverUrl: "/examples/jubilee-zhenya.webp",
  },
  {
    id: "jubilee",
    title: "Юбилей",
    category: "Юбилей",
    description:
      "Торжественный трек к круглой дате — о пройденном пути, достижениях и наступающем новом этапе жизни.",
    mood: "Торжество",
    audioUrl: "/examples/jubilee.mp3",
    coverUrl: "/examples/jubilee.webp",
  },
  {
    id: "tatyana",
    title: "Колесниковой Татьяне",
    category: "Подарок",
    description:
      "Именная песня в подарок Татьяне — нежное посвящение, созданное специально для одного-единственного человека.",
    mood: "Тепло",
    audioUrl: "/examples/tatyana.mp3",
    coverUrl: "/examples/tatyana.webp",
  },
  {
    id: "big-house",
    title: "Наш большой дом",
    category: "Новоселье",
    description:
      "Светлая песня о родном доме и уюте — отличный подарок на новоселье и для всей семьи.",
    mood: "Тепло",
    audioUrl: "/examples/big-house.mp3",
    coverUrl: "/examples/big-house.webp",
  },
];
