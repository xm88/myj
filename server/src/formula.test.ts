/** 旧版公式自测：固定伤害、成长与属性样例，供 Go 迁移数值对照。 */

/**
 * 公式的最小自测。不连数据库，改战斗数值时先跑 `npm test`。
 */
import assert from 'node:assert/strict'
import { applyAttributeBonuses, attributeBudget, computeDamage, defenseReduction, emptyAttributes, expToNext, totalAttributes } from './formula.ts'

const reduction = defenseReduction(500)
assert.ok(Math.abs(reduction - 0.5) < 1e-9, '防御 500 时应减伤 50%')

const normal = computeDamage({
  attack: 100,
  skillCoefficient: 1,
  defense: 0,
  elementBonus: 0,
  elementResist: 0,
  critRate: 0,
  critDmg: 1.5,
  rng: () => 0.99,
})
assert.equal(normal.amount, 100)
assert.equal(normal.crit, false)

const crit = computeDamage({
  attack: 100,
  skillCoefficient: 2,
  defense: 500,
  elementBonus: 0.2,
  elementResist: 0,
  critRate: 0,
  critDmg: 1.5,
  forceCrit: true,
})
// 100 * 2 * 0.5 * 1.2 * 1.5 = 180
assert.equal(crit.amount, 180)
assert.equal(crit.crit, true)

const resist = computeDamage({
  attack: 100,
  skillCoefficient: 1,
  defense: 0,
  elementBonus: 0,
  elementResist: 0.5,
  critRate: 0,
  critDmg: 1.5,
  rng: () => 1,
})
assert.equal(resist.amount, 50)

assert.equal(attributeBudget(1), 5)
assert.equal(attributeBudget(30), 150)
const grown = { maxHp: 100, maxMp: 40, atk: 10, def: 4, spell: 0, attackInterval: 1100, speed: 170 }
const attrs = emptyAttributes()
attrs.vit = 2
attrs.str = 1
attrs.agi = 10
attrs.end = 2
attrs.mag = 3
applyAttributeBonuses(grown, attrs)
assert.equal(grown.maxHp, 110)
assert.equal(grown.maxMp, 49)
assert.ok(Math.abs(grown.atk - 10.66) < 1e-9)
assert.equal(grown.def, 7)
assert.ok(Math.abs(grown.spell - 3.5) < 1e-9)
assert.ok(Math.abs(grown.attackInterval - 1070) < 1e-9)
assert.ok(Math.abs(grown.speed - 176) < 1e-9)
const capped = { maxHp: 1, maxMp: 1, atk: 1, def: 1, spell: 0, attackInterval: 700, speed: 1 }
applyAttributeBonuses(capped, { ...emptyAttributes(), agi: 80 })
assert.equal(capped.attackInterval, 650)
assert.equal(capped.spell, 0)
assert.equal(totalAttributes(10, emptyAttributes(), 'human').vit, 10)
assert.equal(totalAttributes(10, emptyAttributes(), 'demon').vit, 20)
assert.equal(totalAttributes(10, emptyAttributes(), 'demon').end, 0)
assert.equal(totalAttributes(10, emptyAttributes(), 'immortal').mag, 20)
assert.equal(totalAttributes(10, emptyAttributes(), 'immortal').vit, 0)
const demonBlood = { maxHp: 0, maxMp: 0, atk: 0, def: 0, spell: 0, attackInterval: 1100, speed: 170 }
applyAttributeBonuses(demonBlood, { ...emptyAttributes(), vit: 1 }, 'demon')
assert.equal(demonBlood.maxHp, 6)
const immortalGuard = { maxHp: 0, maxMp: 0, atk: 0, def: 0, spell: 0, attackInterval: 1100, speed: 170 }
applyAttributeBonuses(immortalGuard, { ...emptyAttributes(), end: 1 }, 'immortal')
assert.equal(immortalGuard.def, 1.6)

assert.ok(expToNext(1) < expToNext(10))
assert.ok(expToNext(10) < expToNext(30))

console.log('formula ok', { exp1: expToNext(1), exp10: expToNext(10), exp30: expToNext(30), points: attributeBudget(1) })
