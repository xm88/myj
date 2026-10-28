/** 旧版 TypeScript 迁移参考：背包、装备、仓库和消耗品。默认服务启动执行 server/*.go。 */

/**
 * 背包、仓库、装备和消耗品。
 * 可堆叠材料和装备实例存在同一张表里。装备的品质和词条在 extra 里，
 * 由服务器在掉落或打造时掷出，客户端只能说“我要穿上这一件”。
 */

import type { App } from './app.ts'
import { many, one, run } from './db.ts'
import { emit } from './events.ts'
import { QUALITY_TABLE } from './formula.ts'
import { sendMail } from './mail.ts'
import { notify } from './net.ts'
import { defaultLoadout } from './player.ts'
import { refreshStats } from './stats.ts'
import type { ItemExtra, Quality } from './types.ts'
import { GameError, now, uid } from './util.ts'

/** 数据库物品实例记录，保存归属、模板、数量、位置和随机词条。 */
export interface ItemRow {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 记录所属角色标识，业务查询同时校验归属。 */
  player_id: string
  /** 关联配置模板的标识，实例数值和模板规则分开保存。 */
  template_id: string
  /** 堆叠数量或任务目标所需次数。 */
  count: number
  /** 物品当前位于背包、仓库、装备或其他业务位置。 */
  location: string
  /** 装备实例的品质与词条 JSON，普通堆叠材料为空。 */
  extra: string | null
  /** 记录创建时间，使用 Unix 毫秒时间戳。 */
  created_at: number
}

/** 按品质抽取不重复的装备词条并掷出对应数值。 */
export function makeEquipmentExtra(app: App, quality: Quality): ItemExtra {
  const count = QUALITY_TABLE[quality]?.affixes ?? 0
  const pool = [...app.data.affixes]
  const affixes: ItemExtra['affixes'] = []
  for (let i = 0; i < count && pool.length > 0; i++) {
    const index = Math.floor(Math.random() * pool.length)
    const def = pool.splice(index, 1)[0]!
    let value = def.min + Math.random() * (def.max - def.min)
    if (def.stat === 'critRate' || def.stat === 'critDmg' || def.stat.startsWith('elem_')) {
      value = Math.round(value * 1000) / 1000
    } else {
      value = Math.max(1, Math.round(value))
    }
    affixes.push({ id: def.id, name: def.name, stat: def.stat, value })
  }
  return { quality, affixes }
}

/** 查询角色物品，可按背包、装备或仓库位置过滤。 */
export function listItems(app: App, playerId: string, location?: string): ItemRow[] {
  if (location) return many<ItemRow>(app.db, 'SELECT * FROM player_items WHERE player_id = ? AND location = ?', playerId, location)
  return many<ItemRow>(app.db, 'SELECT * FROM player_items WHERE player_id = ?', playerId)
}

/** 统计背包中普通堆叠材料的数量，不计算装备实例和仓库物品。 */
export function countItem(app: App, playerId: string, templateId: string): number {
  const rows = many<{ count: number }>(
    app.db,
    `SELECT count FROM player_items WHERE player_id = ? AND template_id = ? AND location = 'bag' AND extra IS NULL`,
    playerId,
    templateId,
  )
  return rows.reduce((sum, row) => sum + row.count, 0)
}

/** 统计某个存放位置占用的物品行数，用于容量判断。 */
function slots(app: App, playerId: string, location: string): number {
  const row = one<{ n: number }>(app.db, 'SELECT COUNT(*) AS n FROM player_items WHERE player_id = ? AND location = ?', playerId, location)
  return Number(row?.n ?? 0)
}

/** 材料合并堆叠、装备逐件建实例；空间不足时将剩余奖励寄到邮箱。 */
export function addItem(
  app: App,
  playerId: string,
  templateId: string,
  count: number,
  extra?: ItemExtra | null,
  location: 'bag' | 'bank' | 'equipped' = 'bag',
) {
  const tpl = app.items.get(templateId)
  if (!tpl || count <= 0) return
  const cap = location === 'bank' ? app.data.constants.bankCap : app.data.constants.bagCap
  const instanced = tpl.kind === 'equipment' || Boolean(extra)

  if (instanced) {
    for (let i = 0; i < count; i++) {
      if (location !== 'equipped' && slots(app, playerId, location) >= cap) {
        sendMail(app, playerId, '背包满了', `${tpl.name} 先寄存在邮箱。`, {
          items: [{ templateId, count: count - i, extra: extra ?? null }],
        })
        return
      }
      run(
        app.db,
        'INSERT INTO player_items(id, player_id, template_id, count, location, extra, created_at) VALUES(?, ?, ?, 1, ?, ?, ?)',
        uid(),
        playerId,
        templateId,
        location,
        extra ? JSON.stringify(extra) : JSON.stringify(makeEquipmentExtra(app, 'white')),
        now(),
      )
    }
  } else {
    const existing = one<ItemRow>(
      app.db,
      'SELECT * FROM player_items WHERE player_id = ? AND template_id = ? AND location = ? AND extra IS NULL',
      playerId,
      templateId,
      location,
    )
    if (existing) {
      run(app.db, 'UPDATE player_items SET count = ? WHERE id = ?', existing.count + count, existing.id)
    } else if (location !== 'equipped' && slots(app, playerId, location) >= cap) {
      sendMail(app, playerId, '背包满了', `${tpl.name} x${count} 先寄存在邮箱。`, {
        items: [{ templateId, count, extra: null }],
      })
      return
    } else {
      run(
        app.db,
        'INSERT INTO player_items(id, player_id, template_id, count, location, extra, created_at) VALUES(?, ?, ?, ?, ?, NULL, ?)',
        uid(),
        playerId,
        templateId,
        count,
        location,
        now(),
      )
    }
  }

  if (location === 'bag') emit(app, playerId, { type: 'collect', itemId: templateId, count })
}

/** 按材料模板扣除背包堆叠数量，材料不足时终止业务操作。 */
export function consumeTemplate(app: App, playerId: string, templateId: string, count: number) {
  const row = one<ItemRow>(
    app.db,
    `SELECT * FROM player_items WHERE player_id = ? AND template_id = ? AND location = 'bag' AND extra IS NULL`,
    playerId,
    templateId,
  )
  if (!row || row.count < count) throw new GameError(`${app.items.get(templateId)?.name ?? '材料'}不够`)
  if (row.count === count) run(app.db, 'DELETE FROM player_items WHERE id = ?', row.id)
  else run(app.db, 'UPDATE player_items SET count = ? WHERE id = ?', row.count - count, row.id)
}

/** 按实例 ID 和角色归属读取物品，避免操作其他人的物品。 */
export function takeItem(app: App, playerId: string, itemId: string): ItemRow {
  const row = one<ItemRow>(app.db, 'SELECT * FROM player_items WHERE id = ? AND player_id = ?', itemId, playerId)
  if (!row) throw new GameError('没有这件物品')
  return row
}

/** 从给定实例扣数量，堆叠耗尽时删除记录。 */
function consumeRow(app: App, row: ItemRow, count = 1) {
  if (row.count <= count) run(app.db, 'DELETE FROM player_items WHERE id = ?', row.id)
  else run(app.db, 'UPDATE player_items SET count = ? WHERE id = ?', row.count - count, row.id)
}

/** 替换同部位装备，换武器时同步默认技能栏与在线面板。 */
export function equipItem(app: App, playerId: string, itemId: string) {
  const row = takeItem(app, playerId, itemId)
  if (row.location === 'listed') throw new GameError('它正在拍卖')
  const tpl = app.items.get(row.template_id)
  if (!tpl || tpl.kind !== 'equipment' || !tpl.slot) throw new GameError('这件不能装备')
  if (row.location === 'bank') throw new GameError('先从仓库取出来')

  for (const equipped of listItems(app, playerId, 'equipped')) {
    if (app.items.get(equipped.template_id)?.slot === tpl.slot) {
      run(app.db, `UPDATE player_items SET location = 'bag' WHERE id = ?`, equipped.id)
    }
  }
  run(app.db, `UPDATE player_items SET location = 'equipped' WHERE id = ?`, row.id)

  if (tpl.slot === 'weapon' && tpl.weaponType) {
    const loadout = defaultLoadout(app, tpl.weaponType)
    run(
      app.db,
      'UPDATE players SET weapon_type = ?, skill_loadout = ?, updated_at = ? WHERE id = ?',
      tpl.weaponType,
      JSON.stringify(loadout),
      now(),
      playerId,
    )
    const live = app.live.get(playerId)
    if (live) {
      live.weaponType = tpl.weaponType
      live.loadout = loadout
    }
  }
  refreshStats(app, playerId)
  const quality = row.extra ? (JSON.parse(row.extra) as ItemExtra).quality : 'white'
  emit(app, playerId, { type: 'equip', quality })
  notify(app, playerId)
}

/** 检查背包空间后卸下装备，再更新面板与客户端档案。 */
export function unequipItem(app: App, playerId: string, slot: string) {
  const found = listItems(app, playerId, 'equipped').find((row) => app.items.get(row.template_id)?.slot === slot)
  if (!found) throw new GameError('这个部位是空的')
  if (slots(app, playerId, 'bag') >= app.data.constants.bagCap) throw new GameError('背包满了')
  run(app.db, `UPDATE player_items SET location = 'bag' WHERE id = ?`, found.id)
  refreshStats(app, playerId)
  notify(app, playerId)
}

/** 在背包和仓库间移动实例或部分堆叠材料，并触发仓库任务事件。 */
export function moveStorage(app: App, playerId: string, itemId: string, direction: 'deposit' | 'withdraw', count: number) {
  const row = takeItem(app, playerId, itemId)
  const from = direction === 'deposit' ? 'bag' : 'bank'
  const to = direction === 'deposit' ? 'bank' : 'bag'
  if (row.location !== from) throw new GameError('物品不在这边')
  const tpl = app.items.get(row.template_id)
  if (!tpl) throw new GameError('物品不存在')
  if (tpl.bound) throw new GameError('这件不能放进仓库')

  const cap = to === 'bank' ? app.data.constants.bankCap : app.data.constants.bagCap
  const instanced = tpl.kind === 'equipment' || Boolean(row.extra)
  const amount = instanced ? 1 : Math.min(Math.max(1, count), row.count)

  if (instanced) {
    if (slots(app, playerId, to) >= cap) throw new GameError(to === 'bag' ? '背包满了' : '仓库满了')
    run(app.db, 'UPDATE player_items SET location = ? WHERE id = ?', to, row.id)
  } else {
    const existing = one<ItemRow>(
      app.db,
      'SELECT * FROM player_items WHERE player_id = ? AND template_id = ? AND location = ? AND extra IS NULL',
      playerId,
      row.template_id,
      to,
    )
    if (!existing && slots(app, playerId, to) >= cap) throw new GameError(to === 'bag' ? '背包满了' : '仓库满了')
    if (amount === row.count) run(app.db, 'DELETE FROM player_items WHERE id = ?', row.id)
    else run(app.db, 'UPDATE player_items SET count = ? WHERE id = ?', row.count - amount, row.id)
    if (existing) run(app.db, 'UPDATE player_items SET count = ? WHERE id = ?', existing.count + amount, existing.id)
    else {
      run(
        app.db,
        'INSERT INTO player_items(id, player_id, template_id, count, location, extra, created_at) VALUES(?, ?, ?, ?, ?, NULL, ?)',
        uid(),
        playerId,
        row.template_id,
        amount,
        to,
        now(),
      )
    }
  }
  emit(app, playerId, { type: 'bank' })
  notify(app, playerId)
}

/** 消耗品结算结果，可把需要发放的灵伴交给灵伴模块继续处理。 */
export interface UseResult {
  /** 消耗品要求发放的灵伴模板 ID，没有灵伴效果时为空。 */
  grantPet: string | null
  /** 用于客户端显示的中文名称。 */
  name: string
}

/** 使用消耗品。若是遗迹蛋，返回要契约的灵伴 id，由宠物模块接着处理。 */
export function useItem(app: App, playerId: string, itemId: string): UseResult {
  const row = takeItem(app, playerId, itemId)
  if (row.location !== 'bag') throw new GameError('只能使用背包里的东西')
  const tpl = app.items.get(row.template_id)
  if (!tpl || tpl.kind !== 'consumable') throw new GameError('这件不能直接使用')
  if (tpl.id === 'capture_orb') throw new GameError('把目标打到残血后，用捕捉键契约')

  if (tpl.effect?.grantPet) {
    consumeRow(app, row, 1)
    notify(app, playerId)
    return { grantPet: tpl.effect.grantPet, name: tpl.name }
  }

  const live = app.live.get(playerId)
  if (tpl.effect?.healHp) {
    if (live) live.hp = Math.min(live.stats.maxHp, live.hp + tpl.effect.healHp)
    else run(app.db, 'UPDATE players SET hp = MIN(hp + ?, 9999) WHERE id = ?', tpl.effect.healHp, playerId)
  }
  if (tpl.effect?.healMp && live) live.mp = Math.min(live.stats.maxMp, live.mp + tpl.effect.healMp)
  if (tpl.effect?.buff) {
    const buff = {
      id: tpl.id,
      stat: tpl.effect.buff.stat,
      value: tpl.effect.buff.value,
      until: now() + tpl.effect.buff.ms,
    }
    if (live) {
      live.buffs = live.buffs.filter((item) => item.id !== buff.id)
      live.buffs.push(buff)
      refreshStats(app, playerId)
    }
  }
  consumeRow(app, row, 1)
  notify(app, playerId)
  return { grantPet: null, name: tpl.name }
}

/** 从背包扣除上架数量，复用实例消耗规则。 */
export function removeListed(app: App, row: ItemRow, count: number) {
  consumeRow(app, row, count)
}
