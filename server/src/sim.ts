/** 旧版 TypeScript 迁移参考：分线、移动碰撞、怪物 AI 与战斗快照。默认服务启动执行 server/*.go。 */

/**
 * 世界模拟。
 * 地图分线、移动、怪物和技能都在服务器上跑，一秒十次。
 * 客户端发来的只有意图：方向、目标、技能槽、捕捉、互动。
 * 伤害、掉落、经验和死亡在这里结算。
 */

import type { MapDef } from './content/schema.ts'
import type { App } from './app.ts'
import { one, run } from './db.ts'
import { partyKey } from './economy.ts'
import { emit } from './events.ts'
import { computeDamage, dayPeriod, expToNext, PERIOD_LABEL } from './formula.ts'
import { addItem, consumeTemplate, makeEquipmentExtra } from './inventory.ts'
import { broadcast, send, toast } from './net.ts'
import { addExp, addGold, addTrackExp, getPlayer, saveLive } from './player.ts'
import { addPetExp, givePet, goldRange } from './pets.ts'
import { openDialog } from './quest.ts'
import type { Instance, LivePlayer, MonsterState } from './runtime.ts'
import { computeStats, parseBuffs, refreshStats } from './stats.ts'
import type { Element, Quality, WeaponType } from './types.ts'
import { clamp, dist, GameError, now, uid } from './util.ts'
import type { WebSocket } from 'ws'

/** 世界模拟的固定步长，100ms 对应每秒十次更新。 */
const TICK_MS = 100

/** 恢复或接管角色世界连接，进入分线并推送当前天气时段。 */
export function attachPlayer(app: App, playerId: string, socket: WebSocket) {
  let live = app.live.get(playerId)
  if (!live) {
    const row = getPlayer(app, playerId)
    const weapon = row.weapon_type as WeaponType
    const buffs = parseBuffs(row.buffs).filter((buff) => buff.until > now())
    const stats = computeStats(app, playerId, row.level, weapon, buffs)
    let loadout: string[] = []
    try {
      loadout = JSON.parse(row.skill_loadout) as string[]
    } catch {
      loadout = []
    }
    live = {
      id: row.id,
      userId: row.user_id,
      name: row.name,
      level: row.level,
      exp: row.exp,
      mapId: row.map_id,
      x: row.x,
      y: row.y,
      hp: clamp(row.hp, 1, stats.maxHp),
      mp: clamp(row.mp, 0, stats.maxMp),
      gold: row.gold,
      weaponType: weapon,
      armed: true,
      loadout,
      instanceKey: '',
      inputX: 0,
      inputY: 0,
      destX: null,
      destY: null,
      targetId: null,
      safe: false,
      cd: {},
      invulnUntil: 0,
      shield: 0,
      buffs,
      lastCombatAt: 0,
      attackAt: 0,
      stats,
      eventSeq: 1,
      events: [],
      socket,
      chatAt: 0,
      disconnectAt: 0,
    }
    app.live.set(playerId, live)
    refreshStats(app, playerId)
    enterInstance(app, live, live.mapId, false)
  } else {
    const previous = live.socket
    live.socket = socket
    live.disconnectAt = 0
    live.safe = false
    if (previous && previous !== socket) previous.close(4001, 'replaced')
    const inst = app.instances.get(live.instanceKey)
    if (!inst || !inst.players.has(live.id)) enterInstance(app, live, live.mapId, false)
  }
  const weather = app.data.weathers.find((item) => item.id === app.weather.id)
  send(live, {
    type: 'world',
    weather,
    period: app.period,
    periodName: PERIOD_LABEL[app.period],
  })
}

/** 当前连接断开时停止移动、开启安全状态并保存角色位置。 */
export function detachPlayer(app: App, playerId: string, socket: WebSocket) {
  const live = app.live.get(playerId)
  if (!live || live.socket !== socket) return
  live.socket = null
  live.disconnectAt = now()
  live.inputX = 0
  live.inputY = 0
  live.destX = null
  live.destY = null
  live.safe = true
  saveLive(app, live)
}

/** 限制方向输入范围并保存可选点击目的地，由服务器循环执行移动。 */
export function setInput(app: App, playerId: string, x: number, y: number, destX?: number, destY?: number) {
  const live = app.live.get(playerId)
  if (!live || live.safe) return
  live.inputX = clamp(x || 0, -1, 1)
  live.inputY = clamp(y || 0, -1, 1)
  if (typeof destX === 'number' && typeof destY === 'number') {
    live.destX = destX
    live.destY = destY
  }
}

/** 在冒险模式下更新攻击或契约目标。 */
export function setTarget(app: App, playerId: string, entityId: string | null) {
  const live = app.live.get(playerId)
  if (!live || live.safe) return
  live.targetId = entityId
}

/** 切换助手安全模式，进入助手时停止移动和自动攻击。 */
export function setMode(app: App, playerId: string, mode: string) {
  const live = app.live.get(playerId)
  if (!live) return
  live.safe = mode === 'assistant'
  if (live.safe) {
    live.inputX = 0
    live.inputY = 0
    live.destX = null
    live.destY = null
    live.targetId = null
  }
}

/** 校验航标使用地点和目的地图，城镇可直接前往解锁地图。 */
export function travel(app: App, playerId: string, mapId: string) {
  const live = mustLive(app, playerId)
  const here = app.maps.get(live.mapId)
  const dest = app.maps.get(mapId)
  if (!here || !dest) throw new GameError('没有这张地图')
  if (live.level < dest.unlockLevel) throw new GameError(`需要 ${dest.unlockLevel} 级`)
  if (here.kind === 'wild') throw new GameError('野外要走传送门，或者先回星落镇')
  if (here.kind === 'dungeon' && mapId !== 'town_starfall') throw new GameError('先离开遗迹')
  enterInstance(app, live, mapId, true)
}

/** 处理武器、灵伴与闪避技能，校验条件后执行服务器权威结算。 */
export function castSkill(app: App, playerId: string, slot: number | 'dodge' | 'pet') {
  const live = app.live.get(playerId)
  if (!live || live.safe) return
  const t = now()
  if (slot === 'dodge') {
    if ((live.cd.dodge ?? 0) > t) return
    live.cd.dodge = t + 4000
    live.invulnUntil = t + 650
    const len = Math.hypot(live.inputX, live.inputY) || 1
    const map = app.maps.get(live.mapId)!
    const moved = slide(map, live.x, live.y, (-live.inputX / len) * 46, (-live.inputY / len) * 46, 14)
    live.x = moved.x
    live.y = moved.y
    push(live, { kind: 'text', text: '闪避', x: live.x, y: live.y })
    return
  }

  let skillId = ''
  if (slot === 'pet') {
    const pet = one<{ template_id: string }>(app.db, 'SELECT template_id FROM player_pets WHERE player_id = ? AND active = 1', playerId)
    if (!pet) {
      toast(app, playerId, '没有主战灵伴')
      return
    }
    skillId = app.pets.get(pet.template_id)?.skillId ?? ''
  } else {
    if (!live.armed) {
      toast(app, playerId, '没有装备武器')
      return
    }
    skillId = live.loadout[slot - 1] ?? ''
  }
  const skill = app.skills.get(skillId)
  if (!skill) return
  if ((live.cd[skill.id] ?? 0) > t) return
  if (live.mp < skill.mp) {
    toast(app, playerId, '法力不足')
    return
  }
  const inst = app.instances.get(live.instanceKey)
  if (!inst) return
  const target = live.targetId ? inst.monsters.get(live.targetId) : undefined
  if (skill.coefficient > 0 && !skill.aoe) {
    if (!target?.alive) {
      toast(app, playerId, '先选中一个目标')
      return
    }
    const tmpl = app.monsters.get(target.templateId)!
    if (dist(live.x, live.y, target.x, target.y) > skill.range + tmpl.radius) {
      toast(app, playerId, '距离不够')
      return
    }
  }

  live.mp = Math.max(0, live.mp - skill.mp)
  live.cd[skill.id] = t + skill.cooldown
  if (skill.buff) {
    live.buffs = live.buffs.filter((buff) => buff.id !== skill.id)
    live.buffs.push({ id: skill.id, stat: skill.buff.stat, value: skill.buff.value, until: t + skill.buff.ms })
    refreshStats(app, live.id)
  }
  if (skill.shieldFlat) live.shield += skill.shieldFlat
  if (skill.healPct) {
    const amount = Math.max(1, Math.round(live.stats.maxHp * skill.healPct * live.stats.healPct))
    live.hp = Math.min(live.stats.maxHp, live.hp + amount)
    push(live, { kind: 'heal', amount, x: live.x, y: live.y, entityId: live.id })
  }

  const victims: MonsterState[] = []
  if (skill.coefficient > 0) {
    if (skill.aoe) {
      for (const monster of inst.monsters.values()) {
        if (!monster.alive) continue
        const tmpl = app.monsters.get(monster.templateId)!
        if (dist(live.x, live.y, monster.x, monster.y) <= skill.range + tmpl.radius) victims.push(monster)
      }
    } else if (target?.alive) victims.push(target)
  }
  for (const monster of victims) {
    if (skill.tauntMs) {
      monster.targetId = live.id
      monster.tauntUntil = t + skill.tauntMs
    }
    if (skill.slowMs) monster.slowUntil = t + skill.slowMs
    if (skill.shredMs) monster.shredUntil = t + skill.shredMs
    if (skill.vulnerableMs) monster.vulnerableUntil = t + skill.vulnerableMs
    hurtMonster(app, inst, live, monster, skill.coefficient, skill.element, Boolean(skill.forceCrit))
  }
  if (skill.coefficient > 0 || skill.healPct) live.lastCombatAt = t
}

/** 检查可契约目标、残血、时段和距离，消耗灵契球后掷出契约结果。 */
export function tryCapture(app: App, playerId: string) {
  const live = mustLive(app, playerId)
  if (live.safe) throw new GameError('助手模式下不能捕捉')
  const inst = app.instances.get(live.instanceKey)
  const monster = live.targetId ? inst?.monsters.get(live.targetId) : undefined
  if (!monster?.alive) throw new GameError('先选中目标')
  const tmpl = app.monsters.get(monster.templateId)
  if (!tmpl?.petId || tmpl.captureBase == null) throw new GameError('它不会回应契约')
  const hpPct = monster.hp / monster.maxHp
  if (hpPct > 0.3) throw new GameError('打到三成血以下再用灵契球')
  if (tmpl.captureTime && !tmpl.captureTime.includes(app.period)) throw new GameError('这个时辰它不会停下')
  if (dist(live.x, live.y, monster.x, monster.y) > 90) throw new GameError('再靠近一点')
  consumeTemplate(app, playerId, 'capture_orb', 1)
  let chance = tmpl.captureBase + (0.3 - hpPct) * 0.4
  const bonus = tmpl.captureWeather?.[app.weather.id] ?? 0
  chance = clamp(chance + bonus, 0.06, 0.92)
  if (Math.random() > chance) {
    toast(app, playerId, '契约失败，灵契球碎了')
    push(live, { kind: 'text', text: '契约失败', x: monster.x, y: monster.y })
    return
  }
  monster.alive = false
  monster.hp = 0
  monster.deadAt = now()
  monster.respawnAt = tmpl.respawnMs > 0 ? now() + tmpl.respawnMs : 0
  givePet(app, playerId, tmpl.petId, true)
  push(live, { kind: 'text', text: '契约成功', x: monster.x, y: monster.y })
}

/** 从附近 NPC、资源点和传送门中选取目标；指定 focus 时只匹配该互动对象。 */
export function interact(app: App, playerId: string, focus?: string) {
  const live = mustLive(app, playerId)
  if (live.safe) throw new GameError('切回冒险才能互动')
  const map = app.maps.get(live.mapId)
  const inst = app.instances.get(live.instanceKey)
  if (!map || !inst) return
  if (focus) {
    interactFocus(app, live, map, inst, focus.slice(0, 80))
    return
  }
  const options: { d: number; run: () => void }[] = []
  for (const npc of app.data.npcs) {
    if (npc.mapId !== map.id) continue
    const d = dist(live.x, live.y, npc.x, npc.y)
    if (d <= 72) {
      options.push({
        d,
        run: () => {
          live.targetId = null
          send(live, { type: 'dialog', ...openDialog(app, playerId, npc.id) })
        },
      })
    }
  }
  for (const portal of map.portals) {
    const d = dist(live.x, live.y, portal.x, portal.y)
    if (d <= 58) options.push({ d, run: () => usePortal(app, live, portal) })
  }
  for (const node of map.nodes) {
    const d = dist(live.x, live.y, node.x, node.y)
    if (d <= 48) options.push({ d, run: () => gather(app, live, inst, node) })
  }
  if (options.length === 0) {
    toast(app, playerId, '附近没有可以互动的东西')
    return
  }
  halt(live)
  options.sort((a, b) => a.d - b.d)[0]!.run()
}

/** 按客户端指定的 NPC、资源点或传送门标识精确匹配互动目标。 */
function interactFocus(app: App, live: LivePlayer, map: MapDef, inst: Instance, focus: string) {
  if (focus.startsWith('npc:')) {
    const npc = app.data.npcs.find((item) => item.id === focus.slice(4))
    if (!npc || npc.mapId !== map.id) {
      toast(app, live.id, '那个人不在这里')
      return
    }
    if (dist(live.x, live.y, npc.x, npc.y) > 72) {
      toast(app, live.id, '再走近一点')
      return
    }
    halt(live, true)
    send(live, { type: 'dialog', ...openDialog(app, live.id, npc.id) })
    return
  }
  if (focus.startsWith('node:')) {
    const node = map.nodes.find((item) => item.id === focus.slice(5))
    if (!node) {
      toast(app, live.id, '没有这个采集点')
      return
    }
    if (dist(live.x, live.y, node.x, node.y) > 48) {
      toast(app, live.id, '再走近一点')
      return
    }
    halt(live)
    gather(app, live, inst, node)
    return
  }
  if (focus.startsWith('portal:')) {
    const parts = focus.slice(7).split(',')
    const x = Number(parts[0])
    const y = Number(parts[1])
    const portal = map.portals.find((item) => item.x === x && item.y === y)
    if (!portal) {
      toast(app, live.id, '没有这条路')
      return
    }
    if (dist(live.x, live.y, portal.x, portal.y) > 58) {
      toast(app, live.id, '再走近一点')
      return
    }
    halt(live)
    usePortal(app, live, portal)
    return
  }
  toast(app, live.id, '没有这个互动')
}

/** 检查目的地图等级门槛，再将角色送到新地图入口。 */
function usePortal(app: App, live: LivePlayer, portal: MapDef['portals'][number]) {
  const dest = app.maps.get(portal.to)
  if (!dest) return
  if (live.level < dest.unlockLevel) {
    toast(app, live.id, `需要 ${dest.unlockLevel} 级`)
    return
  }
  enterInstance(app, live, dest.id, true)
}

/** 清除方向输入和点击目的地，可选地取消当前攻击目标。 */
function halt(live: LivePlayer, clearTarget = false) {
  live.inputX = 0
  live.inputY = 0
  live.destX = null
  live.destY = null
  if (clearTarget) live.targetId = null
}

/** 每100ms更新天气、离线保留、分线移动与战斗，最后广播快照并清空已发送事件。 */
export function tick(app: App) {
  const t = now()
  if (t >= app.weather.until) {
    const list = app.data.weathers
    const index = Math.max(0, list.findIndex((item) => item.id === app.weather.id))
    const next = list[(index + 1) % list.length]!
    app.weather = { id: next.id, until: t + app.data.constants.weatherMs }
    run(app.db, 'UPDATE world_state SET value = ? WHERE key = ?', JSON.stringify(app.weather), 'weather')
    broadcast(app, { type: 'world', weather: next, period: app.period, periodName: PERIOD_LABEL[app.period] })
  }
  const period = dayPeriod()
  if (period !== app.period) {
    app.period = period
    const weather = app.data.weathers.find((item) => item.id === app.weather.id)
    broadcast(app, { type: 'world', weather, period, periodName: PERIOD_LABEL[period] })
  }

  for (const live of [...app.live.values()]) {
    if (!live.socket && live.disconnectAt && t - live.disconnectAt > 60_000) {
      saveLive(app, live)
      app.instances.get(live.instanceKey)?.players.delete(live.id)
      app.live.delete(live.id)
    }
  }

  for (const inst of [...app.instances.values()]) {
    stepInstance(app, inst, t)
    if (inst.players.size === 0 && (inst.dungeon || inst.monsters.size === 0)) app.instances.delete(inst.key)
    if (inst.dungeon && inst.players.size === 0) app.instances.delete(inst.key)
  }

  for (const live of app.live.values()) {
    if (!live.socket) continue
    send(live, buildSnapshot(app, live, t))
    live.events = []
  }
}

/** 获取世界中的在线角色状态，不存在时提示先进入世界。 */
export function mustLive(app: App, playerId: string): LivePlayer {
  const live = app.live.get(playerId)
  if (!live) throw new GameError('先进入世界')
  return live
}

/** 选择公开地图分线或队伍副本，更新地图状态并保存角色。 */
function enterInstance(app: App, live: LivePlayer, mapId: string, toSpawn: boolean) {
  const map = app.maps.get(mapId)
  if (!map) throw new GameError('没有这张地图')
  if (live.level < map.unlockLevel) throw new GameError(`需要 ${map.unlockLevel} 级`)
  if (live.instanceKey) app.instances.get(live.instanceKey)?.players.delete(live.id)
  const key = map.kind === 'dungeon' ? `dungeon:${map.id}:${partyKey(app, live.id)}` : pickLine(app, map.id)
  const inst = ensureInstance(app, key, map.id, map.kind === 'dungeon')
  live.instanceKey = key
  live.mapId = map.id
  live.targetId = null
  if (toSpawn) {
    live.x = map.spawn.x
    live.y = map.spawn.y
  }
  live.x = clamp(live.x, 20, map.width - 20)
  live.y = clamp(live.y, 20, map.height - 20)
  if (toSpawn && map.kind === 'town') {
    live.hp = live.stats.maxHp
    live.mp = live.stats.maxMp
  }
  inst.players.add(live.id)
  emit(app, live.id, { type: 'enter', mapId: map.id })
  saveLive(app, live)
}

/** 优先使用人数未满的最小编号公开分线，必要时生成下一分线编号。 */
function pickLine(app: App, mapId: string): string {
  let best: Instance | undefined
  for (const inst of app.instances.values()) {
    if (inst.mapId !== mapId || inst.dungeon) continue
    if (inst.players.size >= app.data.constants.lineCap) continue
    if (!best || inst.line < best.line) best = inst
  }
  if (best) return best.key
  let line = 1
  while (app.instances.has(`${mapId}#${line}`)) line += 1
  return `${mapId}#${line}`
}

/** 复用已有分线，或初始化怪物、采集点和稀有刷新计时。 */
function ensureInstance(app: App, key: string, mapId: string, dungeon: boolean): Instance {
  const existing = app.instances.get(key)
  if (existing) return existing
  const map = app.maps.get(mapId)!
  const line = dungeon ? 0 : Number(key.split('#')[1] ?? 1)
  const inst: Instance = {
    key,
    mapId,
    line,
    dungeon,
    monsters: new Map(),
    players: new Set(),
    nodes: new Map(map.nodes.map((node) => [node.id, 0])),
    rareAt: now() + 20_000,
  }
  for (const spawn of map.spawns) spawnMonster(app, inst, spawn.monsterId, spawn.x, spawn.y)
  app.instances.set(key, inst)
  return inst
}

/** 按模板生成怪物实例，初始化出生点、血量、冷却与控制状态。 */
function spawnMonster(app: App, inst: Instance, templateId: string, x: number, y: number) {
  const tmpl = app.monsters.get(templateId)
  if (!tmpl) return
  const id = uid()
  inst.monsters.set(id, {
    id,
    templateId,
    name: tmpl.name,
    x,
    y,
    homeX: x,
    homeY: y,
    hp: tmpl.hp,
    maxHp: tmpl.hp,
    alive: true,
    respawnAt: 0,
    deadAt: 0,
    attackAt: 0,
    targetId: null,
    tauntUntil: 0,
    slowUntil: 0,
    shredUntil: 0,
    vulnerableUntil: 0,
    heavyCount: 0,
  })
}

/** 推进分线内怪物重生、稀有刷新、玩家移动、恢复与双方自动攻击。 */
function stepInstance(app: App, inst: Instance, t: number) {
  const map = app.maps.get(inst.mapId)
  if (!map) return
  const dt = TICK_MS / 1000

  for (const monster of [...inst.monsters.values()]) {
    if (monster.alive) continue
    if (monster.respawnAt > 0 && t >= monster.respawnAt) {
      const tmpl = app.monsters.get(monster.templateId)!
      monster.alive = true
      monster.hp = tmpl.hp
      monster.x = monster.homeX
      monster.y = monster.homeY
      monster.targetId = null
      monster.respawnAt = 0
    } else if (monster.respawnAt === 0 && t - monster.deadAt > 1600) {
      inst.monsters.delete(monster.id)
    }
  }

  if (t >= inst.rareAt) {
    inst.rareAt = t + 45_000
    rollRares(app, inst, map)
  }

  for (const playerId of inst.players) {
    const live = app.live.get(playerId)
    if (!live) continue
    const expired = live.buffs.some((buff) => buff.until <= t)
    if (expired) {
      live.buffs = live.buffs.filter((buff) => buff.until > t)
      refreshStats(app, live.id)
    }
    movePlayer(map, live, dt)
    live.mp = Math.min(live.stats.maxMp, live.mp + 4 * dt)
    if (t - live.lastCombatAt > 6000) {
      live.hp = Math.min(live.stats.maxHp, live.hp + live.stats.maxHp * 0.025 * dt)
    }
    if (live.targetId && !inst.monsters.get(live.targetId)?.alive) live.targetId = null
    if (!live.safe && live.targetId && t >= live.attackAt) {
      const monster = inst.monsters.get(live.targetId)
      if (monster?.alive) {
        const tmpl = app.monsters.get(monster.templateId)!
        if (dist(live.x, live.y, monster.x, monster.y) <= live.stats.range + tmpl.radius) {
          live.attackAt = t + live.stats.attackInterval
          hurtMonster(app, inst, live, monster, 1, 'physical', false)
        }
      }
    }
  }

  for (const monster of inst.monsters.values()) {
    if (!monster.alive) continue
    thinkMonster(app, inst, map, monster, t, dt)
  }
}

/** 根据键盘方向或点击目的地计算位移，并处理地图碰撞。 */
function movePlayer(map: MapDef, live: LivePlayer, dt: number) {
  let ax = live.inputX
  let ay = live.inputY
  const len = Math.hypot(ax, ay)
  if (len > 1) {
    ax /= len
    ay /= len
  }
  if (len > 0.08) {
    live.destX = null
    live.destY = null
  } else if (live.destX != null && live.destY != null) {
    const dx = live.destX - live.x
    const dy = live.destY - live.y
    const d = Math.hypot(dx, dy)
    if (d < 8) {
      live.destX = null
      live.destY = null
      ax = 0
      ay = 0
    } else {
      ax = dx / d
      ay = dy / d
    }
  }
  if (ax === 0 && ay === 0) return
  const moved = slide(map, live.x, live.y, ax * live.stats.speed * dt, ay * live.stats.speed * dt, 14)
  live.x = moved.x
  live.y = moved.y
}

/** 处理怪物仇恨、追击、攻击和脱战回位。 */
function thinkMonster(app: App, inst: Instance, map: MapDef, monster: MonsterState, t: number, dt: number) {
  const tmpl = app.monsters.get(monster.templateId)
  if (!tmpl) return
  const target = monster.targetId ? app.live.get(monster.targetId) : undefined
  const taunted = monster.tauntUntil > t && target && !target.safe && target.instanceKey === inst.key
  if (!taunted) {
    if (!target || target.safe || target.instanceKey !== inst.key || dist(monster.x, monster.y, target.x, target.y) > tmpl.leash) {
      monster.targetId = nearestPlayer(app, inst, monster, tmpl.aggro)
    }
  }
  const player = monster.targetId ? app.live.get(monster.targetId) : undefined
  if (!player || player.safe || player.instanceKey !== inst.key) {
    const home = dist(monster.x, monster.y, monster.homeX, monster.homeY)
    if (home > 8) {
      const moved = slide(map, monster.x, monster.y, ((monster.homeX - monster.x) / home) * 70 * dt, ((monster.homeY - monster.y) / home) * 70 * dt, tmpl.radius)
      monster.x = moved.x
      monster.y = moved.y
    }
    if (monster.hp < monster.maxHp && home < 12) monster.hp = monster.maxHp
    return
  }
  if (dist(monster.x, monster.y, monster.homeX, monster.homeY) > tmpl.leash + 40) {
    monster.x = monster.homeX
    monster.y = monster.homeY
    monster.hp = monster.maxHp
    monster.targetId = null
    return
  }
  const gap = dist(monster.x, monster.y, player.x, player.y)
  const slow = monster.slowUntil > t ? 0.55 : 1
  if (gap > tmpl.range * 0.85) {
    const moved = slide(map, monster.x, monster.y, ((player.x - monster.x) / gap) * 90 * slow * dt, ((player.y - monster.y) / gap) * 90 * slow * dt, tmpl.radius)
    monster.x = moved.x
    monster.y = moved.y
  } else if (t >= monster.attackAt) {
    monster.attackAt = t + tmpl.interval
    monster.heavyCount += 1
    const heavy = tmpl.heavyEvery ? monster.heavyCount % tmpl.heavyEvery === 0 : false
    hurtPlayer(app, inst, monster, player, heavy ? 1.8 : 1)
  }
}

/** 在仇恨范围内寻找最近的非安全模式角色。 */
function nearestPlayer(app: App, inst: Instance, monster: MonsterState, aggro: number): string | null {
  let best: string | null = null
  let bestD = aggro
  for (const playerId of inst.players) {
    const live = app.live.get(playerId)
    if (!live || live.safe) continue
    const d = dist(monster.x, monster.y, live.x, live.y)
    if (d < bestD) {
      bestD = d
      best = playerId
    }
  }
  return best
}

/** 由服务器计算角色对怪物的伤害，记录熟练与事件，血量耗尽时结算击杀。 */
function hurtMonster(
  app: App,
  inst: Instance,
  live: LivePlayer,
  monster: MonsterState,
  coefficient: number,
  element: Element,
  forceCrit: boolean,
) {
  if (!monster.alive) return
  const tmpl = app.monsters.get(monster.templateId)
  if (!tmpl) return
  const t = now()
  let defense = tmpl.def
  if (monster.shredUntil > t) defense *= 0.72
  const result = computeDamage({
    attack: live.stats.atk + (element === 'physical' ? 0 : live.stats.spell),
    skillCoefficient: coefficient,
    defense,
    elementBonus: live.stats.elem[element] ?? 0,
    elementResist: tmpl.elemRes?.[element] ?? 0,
    critRate: live.stats.critRate,
    critDmg: live.stats.critDmg,
    forceCrit,
    takenMul: monster.vulnerableUntil > t ? 1.15 : 1,
  })
  monster.hp -= result.amount
  live.lastCombatAt = t
  if (!monster.targetId) monster.targetId = live.id
  tell(app, inst, { kind: 'dmg', amount: result.amount, crit: result.crit, x: monster.x, y: monster.y, entityId: monster.id })
  if (tmpl.role === 'world') emit(app, live.id, { type: 'world_boss' })
  addTrackExp(app, live.id, `weapon:${live.weaponType}`, 1, 'weapon')
  if (monster.hp <= 0) killMonster(app, inst, live, monster)
}

/** 计算怪物伤害、天气倍率、闪避与护盾；倒地后送回当前地图入口。 */
function hurtPlayer(app: App, inst: Instance, monster: MonsterState, live: LivePlayer, coefficient: number) {
  const t = now()
  if (live.invulnUntil > t || live.safe) {
    tell(app, inst, { kind: 'miss', text: '闪避', x: live.x, y: live.y, entityId: live.id })
    return
  }
  const tmpl = app.monsters.get(monster.templateId)
  if (!tmpl) return
  let attack = tmpl.atk
  if (tmpl.element === 'thunder' && app.weather.id === 'storm') attack *= 1.2
  if (tmpl.nightPower && (app.period === 'night' || app.period === 'late')) attack *= 1.25
  if (app.weather.id === 'blood_moon') attack *= 1.08
  const result = computeDamage({
    attack,
    skillCoefficient: coefficient,
    defense: live.stats.def,
    elementBonus: 0,
    elementResist: live.stats.elemRes[tmpl.element] ?? 0,
    critRate: 0.05,
    critDmg: 1.5,
  })
  let amount = result.amount
  if (live.shield > 0) {
    const absorbed = Math.min(live.shield, amount)
    live.shield -= absorbed
    amount -= absorbed
  }
  live.hp -= amount
  live.lastCombatAt = t
  tell(app, inst, { kind: 'dmg', amount, crit: result.crit, x: live.x, y: live.y, entityId: live.id })
  if (live.hp <= 0) {
    const map = app.maps.get(live.mapId)!
    live.hp = Math.max(1, Math.floor(live.stats.maxHp * 0.35))
    live.x = map.spawn.x
    live.y = map.spawn.y
    live.targetId = null
    live.shield = 0
    live.buffs = []
    live.invulnUntil = t + 4000
    for (const other of inst.monsters.values()) {
      if (other.targetId !== live.id) continue
      other.targetId = null
      other.x = other.homeX
      other.y = other.homeY
      other.hp = other.maxHp
    }
    refreshStats(app, live.id)
    toast(app, live.id, '你被击倒了，退回到入口')
    push(live, { kind: 'text', text: '被击倒', x: live.x, y: live.y })
  }
}

/** 标记怪物死亡与重生时间，结算经验、金币、队伍分享、掉落和击杀任务。 */
function killMonster(app: App, inst: Instance, live: LivePlayer, monster: MonsterState) {
  const tmpl = app.monsters.get(monster.templateId)
  if (!tmpl) return
  monster.alive = false
  monster.hp = 0
  monster.deadAt = now()
  monster.respawnAt = tmpl.respawnMs > 0 ? now() + tmpl.respawnMs : 0
  if (live.targetId === monster.id) live.targetId = null
  addExp(app, live.id, tmpl.exp)
  addGold(app, live.id, goldRange(tmpl.goldMin, tmpl.goldMax))
  const pet = one<{ id: string }>(app.db, 'SELECT id FROM player_pets WHERE player_id = ? AND active = 1', live.id)
  if (pet) addPetExp(app, pet.id, 8)
  const partyId = app.playerParty.get(live.id)
  const party = partyId ? app.parties.get(partyId) : undefined
  if (party) {
    for (const memberId of party.memberIds) {
      if (memberId === live.id) continue
      const mate = app.live.get(memberId)
      if (!mate || mate.instanceKey !== live.instanceKey) continue
      if (dist(mate.x, mate.y, live.x, live.y) > 800) continue
      addExp(app, memberId, Math.round(tmpl.exp * 0.5))
    }
  }
  for (const drop of tmpl.drops) {
    let chance = drop.chance
    if (drop.weatherBoost && drop.weatherBoost === app.weather.id) chance = Math.min(0.95, chance * 2)
    if (Math.random() > chance) continue
    const count = Math.max(1, Math.round(drop.min + Math.random() * (drop.max - drop.min)))
    const item = app.items.get(drop.itemId)
    if (!item) continue
    if (item.kind === 'equipment') {
      addItem(app, live.id, item.id, 1, makeEquipmentExtra(app, rollQuality(drop)))
      push(live, { kind: 'loot', text: `获得 ${item.name}`, x: monster.x, y: monster.y })
    } else {
      addItem(app, live.id, item.id, count)
      push(live, { kind: 'loot', text: `获得 ${item.name} x${count}`, x: monster.x, y: monster.y })
    }
  }
  emit(app, live.id, { type: 'kill', monsterId: tmpl.id })
  tell(app, inst, { kind: 'text', text: `${tmpl.name} 被击败`, x: monster.x, y: monster.y })
}

/** 按固定品质或配置权重决定装备掉落品质。 */
function rollQuality(drop: { quality?: Quality; qualities?: { quality: Quality; weight: number }[] }): Quality {
  if (drop.quality) return drop.quality
  const list = drop.qualities
  if (!list?.length) return 'white'
  let cursor = Math.random() * list.reduce((sum, item) => sum + item.weight, 0)
  for (const item of list) {
    cursor -= item.weight
    if (cursor <= 0) return item.quality
  }
  return 'white'
}

/** 按天气、时段、数量上限和概率生成地图稀有怪物。 */
function rollRares(app: App, inst: Instance, map: MapDef) {
  for (const rare of map.rares) {
    if (rare.weather && !rare.weather.includes(app.weather.id)) continue
    if (rare.time && !rare.time.includes(app.period)) continue
    let alive = 0
    for (const monster of inst.monsters.values()) {
      if (monster.alive && monster.templateId === rare.monsterId) alive += 1
    }
    if (alive >= rare.max) continue
    let chance = rare.chance
    if (rare.monsterId === 'gold_slime' && app.weather.id === 'star_tide') chance *= 1.8
    if (Math.random() <= chance) spawnMonster(app, inst, rare.monsterId, rare.x, rare.y)
  }
}

/** 检查资源点冷却，发放材料并推进采集熟练与任务进度。 */
function gather(app: App, live: LivePlayer, inst: Instance, node: MapDef['nodes'][number]) {
  const t = now()
  const ready = inst.nodes.get(node.id) ?? 0
  if (t < ready) {
    toast(app, live.id, '还要等一会儿')
    return
  }
  inst.nodes.set(node.id, t + node.respawnMs)
  let count = 1
  const pet = one<{ template_id: string }>(app.db, 'SELECT template_id FROM player_pets WHERE player_id = ? AND active = 1', live.id)
  if (pet) {
    const def = app.pets.get(pet.template_id)
    const bonus = ((def?.dispatch[node.kind] ?? 0) + (def?.dispatch.versatile ?? 0)) * 0.35
    if (bonus > 0 && Math.random() < bonus) count += 1
  }
  addItem(app, live.id, node.itemId, count)
  addTrackExp(app, live.id, 'life:gather', 8, 'life')
  emit(app, live.id, { type: 'gather', nodeKind: node.kind })
  const name = app.items.get(node.itemId)?.name ?? node.name
  toast(app, live.id, `采集到 ${name} x${count}`)
}

/** 分别尝试横向和纵向位移，使实体遇到阻挡时可以沿边缘滑动。 */
function slide(map: MapDef, x: number, y: number, dx: number, dy: number, radius: number) {
  let nx = clamp(x + dx, radius, map.width - radius)
  let ny = clamp(y + dy, radius, map.height - radius)
  if (blocked(map, nx, y, radius)) nx = x
  if (blocked(map, nx, ny, radius)) ny = y
  return { x: nx, y: ny }
}

/** 判断角色或怪物圆形碰撞范围是否与地图阻挡矩形相交。 */
function blocked(map: MapDef, x: number, y: number, radius: number) {
  return map.blocks.some((block) => x + radius > block.x && x - radius < block.x + block.w && y + radius > block.y && y - radius < block.y + block.h)
}

/** 给角色加入带递增编号的战斗事件，只保留最近三十条。 */
function push(live: LivePlayer, event: Omit<LivePlayer['events'][number], 'id'>) {
  live.events.push({ ...event, id: live.eventSeq++ })
  if (live.events.length > 30) live.events.splice(0, live.events.length - 30)
}

/** 向当前分线中的所有角色加入同一战斗事件。 */
function tell(app: App, inst: Instance, event: Omit<LivePlayer['events'][number], 'id'>) {
  for (const playerId of inst.players) {
    const live = app.live.get(playerId)
    if (live) push(live, event)
  }
}

/** 只输出地图实体、坐标、战斗状态、事件与冷却，完整背包另走 REST。 */
function buildSnapshot(app: App, live: LivePlayer, t: number) {
  const inst = app.instances.get(live.instanceKey)
  const entities: object[] = []
  if (inst) {
    for (const monster of inst.monsters.values()) {
      if (!monster.alive && t - monster.deadAt > 1500) continue
      const tmpl = app.monsters.get(monster.templateId)
      entities.push({
        id: monster.id,
        kind: 'monster',
        templateId: monster.templateId,
        name: monster.name,
        x: monster.x,
        y: monster.y,
        hp: Math.max(0, monster.hp),
        maxHp: monster.maxHp,
        alive: monster.alive,
        tint: tmpl?.tint ?? '#ccc',
        role: tmpl?.role ?? 'normal',
        radius: tmpl?.radius ?? 16,
        capturable: Boolean(tmpl?.petId),
      })
    }
    for (const playerId of inst.players) {
      if (playerId === live.id) continue
      const other = app.live.get(playerId)
      if (!other) continue
      entities.push({
        id: other.id,
        kind: 'player',
        templateId: other.weaponType,
        name: other.name,
        x: other.x,
        y: other.y,
        hp: other.hp,
        maxHp: other.stats.maxHp,
        alive: true,
        tint: '#f2d48a',
        role: 'player',
        radius: 14,
      })
    }
  }
  const cooldowns: Record<string, number> = {}
  for (const [id, at] of Object.entries(live.cd)) cooldowns[id] = Math.max(0, at - t)
  return {
    type: 'snapshot',
    t,
    mapId: live.mapId,
    line: inst?.line ?? 1,
    self: {
      x: live.x,
      y: live.y,
      hp: live.hp,
      maxHp: live.stats.maxHp,
      mp: live.mp,
      maxMp: live.stats.maxMp,
      level: live.level,
      exp: live.exp,
      expMax: expToNext(live.level),
      gold: live.gold,
      targetId: live.targetId,
      shield: live.shield,
      weaponType: live.weaponType,
      safe: live.safe,
    },
    entities,
    cooldowns,
    events: live.events,
  }
}

export { TICK_MS }
