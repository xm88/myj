/** 同伴号令。稳打是旧档默认，和原来的吃药线一样。 */

export const companionOrders = [
  { id: 'steady', name: '稳打', hint: '血量偏低再吃药或守' },
  { id: 'press', name: '猛攻', hint: '只在快倒下时才喝药' },
  { id: 'cover', name: '照应', hint: '更早治疗，没药就守' },
  { id: 'focus', name: '集火', hint: '打你正在打的那只' },
] as const
