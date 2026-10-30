/** 地图、对话和战斗共用的平面造型。只描述体型和配色，不读战斗公式。 */

export type Body = 'human' | 'blob' | 'quad' | 'bird' | 'shroom' | 'bloom' | 'crab' | 'coil' | 'block' | 'fish' | 'wisp' | 'whale'
export type Hat = 'none' | 'cap' | 'wide' | 'hood' | 'helm' | 'straw' | 'band' | 'officer' | 'tall'
export type Tool = 'none' | 'sword' | 'great' | 'hammer' | 'staff' | 'bow' | 'book' | 'pack' | 'rod' | 'pick' | 'lantern'
export type Ears = 'none' | 'round' | 'point' | 'long' | 'horn' | 'fin'
export type Bulk = 'slim' | 'normal' | 'wide'
export type Rank = 'normal' | 'elite' | 'boss' | 'world'
export type Face = 'up' | 'down' | 'left' | 'right'

/** 一个角色的可画参数。同一种体型靠帽子、工具、耳朵和配色分开。 */
export interface Look {
  body: Body
  coat: string
  trim: string
  skin: string
  hair: string
  hat: Hat
  cape: boolean
  tool: Tool
  ears: Ears
  bulk: Bulk
  tail: boolean
  fluff: boolean
  snout: 'short' | 'long'
  rank: Rank
}

/** 把六位色往亮或往暗推一截。非法颜色原样返回。 */
export function shadeColor(hex: string, amount: number) {
  const matched = /^#?([0-9a-fA-F]{6})$/.exec(hex.trim())
  if (!matched?.[1]) return hex
  const raw = matched[1]
  const clamp = (value: number) => Math.max(0, Math.min(255, value))
  const channel = (start: number) => clamp(parseInt(raw.slice(start, start + 2), 16) + amount)
  const text = (value: number) => value.toString(16).padStart(2, '0')
  return `#${text(channel(0))}${text(channel(2))}${text(channel(4))}`
}

function look(partial: Partial<Look> & Pick<Look, 'body' | 'coat'>): Look {
  return {
    trim: '#f3e2b8',
    skin: '#f0c8a4',
    hair: '#3a2a18',
    hat: 'none',
    cape: false,
    tool: 'none',
    ears: 'none',
    bulk: 'normal',
    tail: false,
    fluff: false,
    snout: 'short',
    rank: 'normal',
    ...partial,
  }
}

function toolOf(weaponType?: string): Tool {
  if (weaponType === 'greatsword') return 'great'
  if (weaponType === 'staff') return 'staff'
  if (weaponType === 'bow') return 'bow'
  return 'sword'
}

/** 自己：暖棕披风和头带，武器剪影跟着身上的武器走。 */
export function lookForSelf(weaponType?: string): Look {
  return look({
    body: 'human',
    coat: '#d08a3c',
    trim: '#f6e2a8',
    skin: '#f4d2b2',
    hair: '#3a2414',
    hat: 'band',
    cape: true,
    tool: toolOf(weaponType),
  })
}

/** 别的旅人：蓝灰短装，和自己的披风错开。有武器类型时带上对应剪影。 */
export function lookForTraveler(seed = '', weaponType?: string): Look {
  const coats = ['#3d6f99', '#356888', '#4a7eaa', '#2c5d86']
  let index = 0
  for (const ch of seed) index = (index + ch.charCodeAt(0)) % coats.length
  return look({
    body: 'human',
    coat: coats[index] || '#3d6f99',
    trim: '#d7e6f4',
    skin: '#f0c8a4',
    hair: '#241c16',
    hat: 'cap',
    tool: weaponType ? toolOf(weaponType) : 'none',
  })
}

/** 星落镇和沿线的人。每人一套帽子、工具和颜色，不靠随机色点区分。 */
const npcLooks: Record<string, Look> = {
  npc_elder: look({ body: 'human', coat: '#3d4f86', trim: '#e7c56a', skin: '#f3d2b0', hair: '#d9d3c6', hat: 'wide', cape: true, tool: 'staff', bulk: 'slim' }),
  npc_smith: look({ body: 'human', coat: '#6b4632', trim: '#c47a45', skin: '#e0b090', hair: '#2a2112', hat: 'band', tool: 'hammer', bulk: 'wide' }),
  npc_shop: look({ body: 'human', coat: '#e7a15a', trim: '#fff6ea', skin: '#f6d2b8', hair: '#8a3e2a', hat: 'straw', tool: 'pack', bulk: 'slim' }),
  npc_auction: look({ body: 'human', coat: '#7a3e62', trim: '#e7c56a', skin: '#f0d0b4', hair: '#2a1820', hat: 'tall', tool: 'book' }),
  npc_dispatch: look({ body: 'human', coat: '#2f6d62', trim: '#f3e2b8', skin: '#f0c8a4', hair: '#1c2e28', hat: 'officer', cape: true, tool: 'sword' }),
  npc_guild: look({ body: 'human', coat: '#243656', trim: '#a33b2b', skin: '#e8c2a4', hair: '#1a1e28', hat: 'cap', cape: true }),
  npc_bank: look({ body: 'human', coat: '#8a7560', trim: '#3d6b45', skin: '#e4c2a0', hair: '#3a3028', hat: 'cap', tool: 'pack', bulk: 'wide' }),
  npc_travel: look({ body: 'human', coat: '#3d6f8a', trim: '#f0d48a', skin: '#f0d2b4', hair: '#243848', hat: 'hood', cape: true, tool: 'lantern' }),
  npc_ranger: look({ body: 'human', coat: '#2f5a38', trim: '#c4d4a4', skin: '#e8c4a4', hair: '#243024', hat: 'hood', cape: true, tool: 'bow' }),
  npc_verge: look({ body: 'human', coat: '#d7b15a', trim: '#5f8f3e', skin: '#f3d0b0', hair: '#5c3a22', bulk: 'slim' }),
  npc_fisher: look({ body: 'human', coat: '#5c7380', trim: '#d7e2ea', skin: '#e8c8a8', hair: '#6a6a6a', hat: 'straw', tool: 'rod' }),
  npc_miner: look({ body: 'human', coat: '#a85a32', trim: '#8d949c', skin: '#e0b090', hair: '#2a2112', hat: 'helm', tool: 'pick', bulk: 'wide' }),
}

const villager = look({ body: 'human', coat: '#7aaa55', trim: '#f3e2b8', hat: 'none' })
const merchant = look({ body: 'human', coat: '#c4894a', trim: '#fff4e0', hat: 'cap', tool: 'pack' })
const guard = look({ body: 'human', coat: '#2f5a38', trim: '#e7c56a', hat: 'officer', cape: true, tool: 'sword' })

function classify(title: string, panels: string[]) {
  const text = `${title}|${panels.join(',')}`
  if (/shop|market|bank|商|货|拍卖|仓库|店/.test(text)) return merchant
  if (/guild|dispatch|守|巡|卫|接待|官/.test(text)) return guard
  return villager
}

function vary(base: Look, id: string): Look {
  let shift = 0
  for (let i = 0; i < id.length; i++) shift = (shift * 17 + id.charCodeAt(i)) % 21
  return { ...base, coat: shadeColor(base.coat, shift - 10) }
}

/** 已知的人用专属造型，其余按村民、商人、守卫三套模板。 */
export function lookForNpc(id: string, title = '', panels: string[] = []): Look {
  const known = npcLooks[id]
  if (known) return { ...known }
  return vary(classify(title, panels), id)
}

function beast(coat: string, extra: Partial<Look> = {}) {
  return look({ body: 'quad', coat, ears: 'point', tail: true, ...extra })
}

/** 每只怪一种体型。精英和首领的描边由 rank 另加，不改这里的底子。 */
const monsterLooks: Record<string, Look> = {
  slime: look({ body: 'blob', coat: '#6cbf73', trim: '#e7f6d4' }),
  gold_slime: look({ body: 'blob', coat: '#e2c14d', trim: '#fff4c4' }),
  pond_slime: look({ body: 'blob', coat: '#4aa3a8', trim: '#d5f3f2' }),
  spark: look({ body: 'blob', coat: '#7ec8f0', trim: '#f4fbff' }),
  forest_rat: beast('#8a6a45', { ears: 'round', bulk: 'slim', snout: 'short', trim: '#e7cbb0' }),
  mine_rat: beast('#6b5344', { ears: 'round', bulk: 'slim', trim: '#d9c2ae' }),
  hare: beast('#d9cbb8', { ears: 'long', bulk: 'slim', tail: true, trim: '#fff8ea' }),
  forest_cat: beast('#d7a15a', { bulk: 'slim', fluff: true, trim: '#f3e2c4' }),
  moon_cat: beast('#f2e7b0', { bulk: 'slim', fluff: true, trim: '#fffaf0' }),
  thunder_cat: beast('#f2e27a', { bulk: 'slim', fluff: true, trim: '#fff6c8' }),
  boar: beast('#a56b45', { ears: 'horn', tail: false, bulk: 'wide', snout: 'long', trim: '#f0d2b4' }),
  elite_boar: beast('#c45a3a', { ears: 'horn', tail: false, bulk: 'wide', snout: 'long', trim: '#f3e2b8' }),
  wolf: beast('#6d7380', { snout: 'long', trim: '#d5dbe3' }),
  elite_wolf: beast('#3e4654', { snout: 'long', bulk: 'wide', trim: '#c5ccd4' }),
  storm_wolf: beast('#6aa0c8', { snout: 'long', trim: '#d5e6f4' }),
  elite_thunder: beast('#4f8fd6', { snout: 'long', bulk: 'wide', trim: '#e7f2ff' }),
  fire_fox: beast('#e07030', { bulk: 'slim', snout: 'long', fluff: true, trim: '#f2c14a' }),
  mist_fox: beast('#b7c4ce', { bulk: 'slim', snout: 'long', trim: '#eef3f6' }),
  otter: beast('#8a5a3a', { ears: 'round', bulk: 'slim', trim: '#e7cbb0' }),
  lizard: beast('#c46a32', { ears: 'none', bulk: 'slim', snout: 'long', tail: true, trim: '#f0d2b4' }),
  shroom: look({ body: 'shroom', coat: '#c45b8a', trim: '#f4e2c4' }),
  boss_mushroom: look({ body: 'shroom', coat: '#9b3d68', trim: '#f3d0e0', bulk: 'wide' }),
  flower_bud: look({ body: 'bloom', coat: '#e7a0c4', trim: '#7aaa55' }),
  crab: look({ body: 'crab', coat: '#c4553c', trim: '#f3c1b4' }),
  elite_crab: look({ body: 'crab', coat: '#a33b32', trim: '#f3d2b4', bulk: 'wide' }),
  heron: look({ body: 'bird', coat: '#d9e2ea', trim: '#f7fbff', snout: 'long', hair: '#243038' }),
  thunder_bird: look({ body: 'bird', coat: '#f0d36a', trim: '#fff6c8', hair: '#3a3014' }),
  raptor: look({ body: 'bird', coat: '#8a5a44', trim: '#e7cbb0', hair: '#2a1810' }),
  elite_raptor: look({ body: 'bird', coat: '#6d4030', trim: '#e7c56a', bulk: 'wide', hair: '#24140e' }),
  boss_wind: look({ body: 'bird', coat: '#d7e7c8', trim: '#f4ffe8', bulk: 'wide', hair: '#243024' }),
  bat: look({ body: 'bird', coat: '#5c4d73', trim: '#d5cce4', ears: 'point', hair: '#241c30' }),
  fishman: look({ body: 'human', coat: '#3d7ea6', trim: '#b7e0ea', skin: '#8fd0c8', hair: '#1d4a62', ears: 'fin' }),
  elite_koi: look({ body: 'fish', coat: '#e07a4a', trim: '#fff4ea' }),
  boss_serpent: look({ body: 'coil', coat: '#2f6d78', trim: '#d5eef0' }),
  ore_golem: look({ body: 'block', coat: '#7d848c', trim: '#d7dde4' }),
  elite_golem: look({ body: 'block', coat: '#596068', trim: '#e7c56a', bulk: 'wide' }),
  boss_tyrant: look({ body: 'block', coat: '#8a3e2f', trim: '#e7c56a', bulk: 'wide' }),
  gale_sprite: look({ body: 'wisp', coat: '#b7e0c8', trim: '#f4fff8' }),
  boss_storm: look({ body: 'wisp', coat: '#8fd0ff', trim: '#fff8ea' }),
  world_whale: look({ body: 'whale', coat: '#8eb4d4', trim: '#f4fbff' }),
  relic_guard: look({ body: 'human', coat: '#c8b48a', trim: '#f3e2b8', hair: '#5c5140', hat: 'helm', tool: 'sword' }),
  relic_mage: look({ body: 'human', coat: '#7a6bb5', trim: '#e6dcc8', hair: '#2a2140', hat: 'hood', cape: true, tool: 'staff' }),
  elite_relic: look({ body: 'human', coat: '#e6d7a8', trim: '#8d6a2a', hair: '#3a3020', hat: 'helm', cape: true, tool: 'sword' }),
  boss_relic: look({ body: 'wisp', coat: '#e6c56a', trim: '#fff8ea' }),
}

function familyLook(id: string): Look {
  if (id.includes('slime')) return look({ body: 'blob', coat: '#6cbf73', trim: '#e7f6d4' })
  if (id.includes('shroom') || id.includes('mushroom')) return look({ body: 'shroom', coat: '#c45b8a', trim: '#f4e2c4' })
  if (id.includes('bud') || id.includes('flower')) return look({ body: 'bloom', coat: '#e7a0c4', trim: '#7aaa55' })
  if (id.includes('sprite') || id.includes('spirit') || id.includes('storm')) return look({ body: 'wisp', coat: '#b7e0c8', trim: '#f4fff8' })
  if (id.includes('crab')) return look({ body: 'crab', coat: '#c4553c' })
  if (id.includes('koi') || id.includes('fish')) return look({ body: 'fish', coat: '#e07a4a', trim: '#fff4ea' })
  if (id.includes('serpent') || id.includes('snake')) return look({ body: 'coil', coat: '#2f6d78' })
  if (id.includes('golem') || id.includes('tyrant')) return look({ body: 'block', coat: '#7d848c', trim: '#d7dde4' })
  if (id.includes('whale')) return look({ body: 'whale', coat: '#8eb4d4', trim: '#f4fbff' })
  if (id.includes('bird') || id.includes('heron') || id.includes('raptor') || id.includes('bat')) {
    return look({ body: 'bird', coat: '#8a5a44', ears: id.includes('bat') ? 'point' : 'none' })
  }
  if (id.includes('guard')) return look({ body: 'human', coat: '#c8b48a', hat: 'helm', tool: 'sword' })
  if (id.includes('mage')) return look({ body: 'human', coat: '#7a6bb5', hat: 'hood', tool: 'staff', cape: true })
  return beast('#8a6a45')
}

function asRank(role?: string): Rank {
  if (role === 'elite' || role === 'boss' || role === 'world') return role
  return 'normal'
}

/** 怪物造型。配置里的 tint 覆盖主色，品级只加描边，不改数值。 */
export function lookForMonster(id: string, role = '', tint?: string): Look {
  const base = monsterLooks[id] ? { ...monsterLooks[id] } : familyLook(id)
  return { ...base, rank: asRank(role), coat: tint || base.coat }
}

const petAlias: Record<string, string> = {
  slime_pet: 'slime',
  shroom_spirit: 'shroom',
  boar_cub: 'boar',
  wolf_pup: 'wolf',
  flower_spirit: 'flower_bud',
  ember_lizard: 'lizard',
  rock_golem: 'ore_golem',
  storm_bird: 'thunder_bird',
  ancient_sprite: 'gale_sprite',
  koi: 'elite_koi',
}

/** 灵伴沿用对应怪物的体型。骑士犬单独戴一顶盔，避免和狼贴在一起。 */
export function lookForPet(id: string): Look {
  if (id === 'dog_knight') {
    return beast('#d7c4a2', { hat: 'helm', snout: 'long', trim: '#f6e7c4' })
  }
  return lookForMonster(petAlias[id] || id)
}

/** 精英、首领、稀有的短标。普通怪没有。 */
export function rankLabel(rank: Rank) {
  if (rank === 'boss') return '首'
  if (rank === 'elite') return '精'
  if (rank === 'world') return '稀'
  return ''
}
