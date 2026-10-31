/** 素材访问层：统一资源版本、去底色、加载缓存以及地图、实体、道具图标查找。 */

/**
 * Q 版美工。
 * 角色和道具是品红底的立绘，加载时抠成透明；地图和登录图保持整张画面。
 */

const artVersion = '?v=q2'

/** 给素材地址统一附加版本号，让更新后的美术资源绕过浏览器旧缓存。 */
function withVersion<T extends Record<string, string>>(source: T): T {
  const next = {} as T
  for (const key of Object.keys(source) as (keyof T)[]) next[key] = `${source[key]}${artVersion}` as T[keyof T]
  return next
}
/** 素材原始路径清单，由资源键生成可用于画布和界面的地址。 */
const raw = {
  login: '/art/fi-ui-login.jpg',
  emblem: '/art/fi-ui-emblem.jpg',
  town: '/art/fi-map-town.jpg',
  forest: '/art/fi-map-forest.jpg',
  lake: '/art/fi-map-lake.jpg',
  mine: '/art/fi-map-mine.jpg',
  thunder: '/art/fi-map-thunder.jpg',
  wind: '/art/fi-map-wind.jpg',
  ruins: '/art/fi-map-ruins.jpg',
  hero: '/art/fi-actor-hero.jpg',
  elder: '/art/fi-actor-elder.jpg',
  smith: '/art/fi-actor-smith.jpg',
  slime: '/art/fi-actor-slime.jpg',
  cat: '/art/fi-actor-cat.jpg',
  boar: '/art/fi-actor-boar.jpg',
  wolf: '/art/fi-actor-wolf.jpg',
  shroom: '/art/fi-actor-shroom.jpg',
  fox: '/art/fi-actor-fox.jpg',
  crab: '/art/fi-actor-crab.jpg',
  bird: '/art/fi-actor-bird.jpg',
  rat: '/art/fi-actor-rat.jpg',
  golem: '/art/fi-actor-golem.jpg',
  serpent: '/art/fi-actor-serpent.jpg',
  whale: '/art/fi-actor-whale.jpg',
  relic: '/art/fi-actor-relic.jpg',
  tree: '/art/fi-prop-tree.jpg',
  house: '/art/fi-prop-house.jpg',
  lamp: '/art/fi-prop-lamp.jpg',
  portal: '/art/fi-prop-portal.jpg',
  herb: '/art/fi-prop-herb.jpg',
  sword: '/art/fi-icon-sword.jpg',
  great: '/art/fi-icon-great.jpg',
  staff: '/art/fi-icon-staff.jpg',
  bow: '/art/fi-icon-bow.jpg',
} as const

/** 给原始素材路径附加统一版本号后的完整地址清单。 */
const files = withVersion(raw)

/** 可加载美术资源的键，由统一素材清单推导。 */
type ArtKey = keyof typeof raw

/** 把图片中的洋红底色转为透明像素，保留角色与道具主体。 */
function keyMagenta(img: HTMLImageElement): HTMLCanvasElement {
  const canvas = document.createElement('canvas')
  canvas.width = img.naturalWidth
  canvas.height = img.naturalHeight
  const context = canvas.getContext('2d', { willReadFrequently: true })
  if (!context) return canvas
  context.drawImage(img, 0, 0)
  const frame = context.getImageData(0, 0, canvas.width, canvas.height)
  const px = frame.data
  for (let i = 0; i < px.length; i += 4) {
    const r = px[i] ?? 0
    const g = px[i + 1] ?? 0
    const b = px[i + 2] ?? 0
    // JPEG 里的品红会偏红，大约 R196 G2 B125。金色和角色本身的绿、红不会同时这么“蓝”。
    if (g < 90 && r > 140 && b > 70 && r > g + 70 && b > g + 40 && b > r * 0.4) {
      const strength = Math.min(r, b) - g
      px[i + 3] = strength > 80 ? 0 : Math.round(255 * (1 - strength / 80))
    }
  }
  context.putImageData(frame, 0, 0)
  return canvas
}

/** 缓存已经转换为透明图标的数据地址。 */
const iconUrlCache = new Map<string, string>()

/** 物品图标是带品红衬底的圆徽，转成透明 PNG 再给界面用。 */
export function medalUrl(src: string): Promise<string> {
  const cached = iconUrlCache.get(src)
  if (cached) return Promise.resolve(cached)
  return new Promise((resolve) => {
    const img = new Image()
    img.onload = () => {
      const url = keyMagenta(img).toDataURL('image/png')
      iconUrlCache.set(src, url)
      resolve(url)
    }
    img.onerror = () => resolve(src)
    img.src = src
  })
}

/** 地图模板与背景素材之间的对应关系。 */
const mapArt: Record<string, ArtKey> = {
  town_starfall: 'town',
  wild_greenleaf: 'forest',
  wild_moonlake: 'lake',
  wild_redrock: 'mine',
  wild_thunder: 'thunder',
  wild_windhigh: 'wind',
  dungeon_ruins: 'ruins',
}

/** 角色、怪物和灵伴模板与实体素材的对应关系。 */
const actorArt: Record<string, ArtKey> = {
  hero: 'hero',
  slime: 'slime',
  pond_slime: 'slime',
  spark: 'slime',
  gold_slime: 'slime',
  slime_pet: 'slime',
  forest_rat: 'rat',
  mine_rat: 'rat',
  hare: 'rat',
  bat: 'rat',
  shroom: 'shroom',
  flower_bud: 'shroom',
  shroom_spirit: 'shroom',
  flower_spirit: 'shroom',
  boss_mushroom: 'shroom',
  boar: 'boar',
  elite_boar: 'boar',
  boar_cub: 'boar',
  wolf: 'wolf',
  elite_wolf: 'wolf',
  storm_wolf: 'wolf',
  wolf_pup: 'wolf',
  forest_cat: 'cat',
  moon_cat: 'cat',
  thunder_cat: 'cat',
  fire_fox: 'fox',
  mist_fox: 'fox',
  crab: 'crab',
  elite_crab: 'crab',
  otter: 'crab',
  fishman: 'crab',
  elite_koi: 'crab',
  koi: 'crab',
  heron: 'bird',
  thunder_bird: 'bird',
  raptor: 'bird',
  elite_raptor: 'bird',
  gale_sprite: 'bird',
  storm_bird: 'bird',
  boss_wind: 'bird',
  boss_storm: 'bird',
  lizard: 'fox',
  ember_lizard: 'fox',
  ore_golem: 'golem',
  elite_golem: 'golem',
  rock_golem: 'golem',
  boss_tyrant: 'golem',
  boss_serpent: 'serpent',
  world_whale: 'whale',
  relic_guard: 'relic',
  relic_mage: 'relic',
  elite_relic: 'relic',
  boss_relic: 'relic',
  ancient_sprite: 'relic',
  dog_knight: 'wolf',
}

/** 四种武器类型对应的界面图标地址。 */
export const weaponIcon: Record<string, string> = {
  sword_shield: files.sword,
  greatsword: files.great,
  staff: files.staff,
  bow: files.bow,
}

/** 物品模板对应的专用图标地址，未知物品由后备规则处理。 */
const itemIconUrl: Record<string, string> = {
  slime_gel: '/art/fi-item-gel.jpg',
  rat_tail: '/art/fi-item-tail.jpg',
  mushroom_cap: '/art/fi-item-shroom.jpg',
  boar_hide: '/art/fi-item-hide.jpg',
  wolf_fang: '/art/fi-item-fang.jpg',
  herb_green: '/art/fi-prop-herb.jpg',
  wood: '/art/fi-item-wood.jpg',
  fish_silver: '/art/fi-item-fish.jpg',
  moon_dew: '/art/fi-item-dew.jpg',
  iron_ore: '/art/fi-item-iron.jpg',
  copper_ore: '/art/fi-item-copper.jpg',
  silver_ore: '/art/fi-item-silver.jpg',
  thunder_ore: '/art/fi-item-thunder.jpg',
  wind_feather: '/art/fi-item-feather.jpg',
  relic_fragment: '/art/fi-item-fragment.jpg',
  spore_king: '/art/fi-item-spore.jpg',
  serpent_scale: '/art/fi-item-scale.jpg',
  tyrant_horn: '/art/fi-item-horn.jpg',
  storm_core: '/art/fi-item-storm.jpg',
  relic_heart: '/art/fi-item-heart.jpg',
  map_fragment: '/art/fi-item-map.jpg',
  capture_orb: '/art/fi-item-orb.jpg',
  bread: '/art/fi-item-bread.jpg',
  potion_hp: '/art/fi-item-potion.jpg',
  potion_hp2: '/art/fi-item-potion2.jpg',
  potion_mp: '/art/fi-item-tea.jpg',
  food_atk: '/art/fi-item-meat.jpg',
  food_def: '/art/fi-item-soup.jpg',
  food_explore: '/art/fi-item-night.jpg',
  relic_egg: '/art/fi-item-egg.jpg',
}

/** 背包、拍卖、制造用的物品圆徽。同一种武器或部位共用一张图。 */
export function itemIcon(templateId: string): string {
  let path = itemIconUrl[templateId]
  if (!path && templateId.startsWith('wpn_sword')) path = files.sword
  if (!path && templateId.startsWith('wpn_great')) path = files.great
  if (!path && templateId.startsWith('wpn_staff')) path = files.staff
  if (!path && templateId.startsWith('wpn_bow')) path = files.bow
  if (!path && templateId.startsWith('arm_head')) path = '/art/fi-item-helm.jpg'
  if (!path && templateId.startsWith('arm_chest')) path = '/art/fi-item-robe.jpg'
  if (!path && templateId.startsWith('arm_gloves')) path = '/art/fi-item-gloves.jpg'
  if (!path && templateId.startsWith('arm_boots')) path = '/art/fi-item-boots.jpg'
  if (!path && templateId.startsWith('arm_ring')) path = '/art/fi-item-ring.jpg'
  if (!path && templateId === 'temper_grit') path = '/art/fi-item-iron.jpg'
  return versioned(path ?? '/art/fi-prop-herb.jpg')
}

/** 给单个素材路径加上统一缓存版本参数。 */
function versioned(path: string) {
  return path.includes('?') ? path : path + artVersion
}

/** 输出地图缩略图地址，供场景与世界地图展示。 */
export function mapThumb(mapId: string): string {
  const key = mapArt[mapId]
  return key ? files[key] : files.town
}

/** 查找角色或怪物头像地址，未知模板使用史莱姆素材。背包和灵伴名册仍用这些图。 */
export function portrait(templateId: string): string {
  const key = actorArt[templateId] ?? 'slime'
  return files[key]
}
