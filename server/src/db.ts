/** 旧版 TypeScript 迁移参考：SQLite 查询、建表与旧字段兼容。默认服务启动执行 server/*.go。 */

/**
 * SQLite 访问层。
 *
 * V0.1 用 Node 内置 `node:sqlite`，这样 Windows 上不装数据库也能把服跑起来。
 * 表的职责对齐设计文档里的 PostgreSQL 拆分：账号、角色、物品实例、灵伴、派遣、
 * 任务、拍卖、公会、邮件分开存。加点存在角色的 attributes 里，装备词条放在物品实例的 JSON 里。
 * `docs/sql/001_postgres.sql` 是同一模型的 PostgreSQL 版本，给以后迁移用。
 */

import fs from 'node:fs'
import path from 'node:path'
import { DatabaseSync } from 'node:sqlite'

/** 旧版 SQLite 数据库连接类型，供持久化模块共用。 */
export type DB = DatabaseSync

/** 打开 SQLite 文件，启用 WAL 和外键，建立表并补齐旧版字段。 */
export function openDatabase(file: string): DB {
  fs.mkdirSync(path.dirname(file), { recursive: true })
  const db = new DatabaseSync(file)
  db.exec('PRAGMA journal_mode = WAL;')
  db.exec('PRAGMA foreign_keys = ON;')
  db.exec(SCHEMA)
  ensureColumns(db)
  return db
}

/** 检查旧存档玩家表，按需加入属性和种族字段。 */
function ensureColumns(db: DB) {
  const columns = db.prepare('PRAGMA table_info(players)').all() as { name: string }[]
  if (!columns.some((column) => column.name === 'attributes')) {
    db.exec(`ALTER TABLE players ADD COLUMN attributes TEXT NOT NULL DEFAULT '{}'`)
  }
  if (!columns.some((column) => column.name === 'race')) {
    db.exec(`ALTER TABLE players ADD COLUMN race TEXT NOT NULL DEFAULT 'human'`)
  }
}

/** 查询第一条记录，查不到时返回空值。 */
export function one<T extends object>(db: DB, sql: string, ...params: (string | number | null)[]): T | undefined {
  const row = db.prepare(sql).get(...params) as T | undefined
  return row
}

/** 执行参数化查询，返回全部匹配的数据库记录。 */
export function many<T extends object>(db: DB, sql: string, ...params: (string | number | null)[]): T[] {
  return db.prepare(sql).all(...params) as T[]
}

/** 返回本次写入影响的行数，用来判断“抢购时订单是不是还在”。 */
export function run(db: DB, sql: string, ...params: (string | number | null)[]): number {
  const result = db.prepare(sql).run(...params)
  return Number(result.changes ?? 0)
}

/** 立即事务。拍卖成交和领取派遣必须整笔成功或整笔失败。 */
export function transaction<T>(db: DB, fn: () => T): T {
  db.exec('BEGIN IMMEDIATE')
  try {
    const result = fn()
    db.exec('COMMIT')
    return result
  } catch (err) {
    try {
      db.exec('ROLLBACK')
    } catch {
      /* 已经回滚时忽略 */
    }
    throw err
  }
}

/** 旧版 SQLite 建表语句，Go 迁移沿用相同的列名与存档结构。 */
const SCHEMA = `
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  is_guest INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS players (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL UNIQUE,
  level INTEGER NOT NULL,
  exp INTEGER NOT NULL,
  map_id TEXT NOT NULL,
  x REAL NOT NULL,
  y REAL NOT NULL,
  hp REAL NOT NULL,
  mp REAL NOT NULL,
  gold INTEGER NOT NULL,
  weapon_type TEXT NOT NULL,
  skill_loadout TEXT NOT NULL,
  attributes TEXT NOT NULL DEFAULT '{}',
  race TEXT NOT NULL DEFAULT 'human',
  flags TEXT NOT NULL DEFAULT '{}',
  buffs TEXT NOT NULL DEFAULT '[]',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS player_tracks (
  player_id TEXT NOT NULL,
  track TEXT NOT NULL,
  level INTEGER NOT NULL,
  exp INTEGER NOT NULL,
  PRIMARY KEY (player_id, track)
);

CREATE TABLE IF NOT EXISTS player_items (
  id TEXT PRIMARY KEY,
  player_id TEXT NOT NULL,
  template_id TEXT NOT NULL,
  count INTEGER NOT NULL,
  location TEXT NOT NULL,
  extra TEXT,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_items_player ON player_items(player_id, location);

CREATE TABLE IF NOT EXISTS player_pets (
  id TEXT PRIMARY KEY,
  player_id TEXT NOT NULL,
  template_id TEXT NOT NULL,
  level INTEGER NOT NULL,
  exp INTEGER NOT NULL,
  active INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS pet_dispatch (
  id TEXT PRIMARY KEY,
  player_id TEXT NOT NULL,
  player_pet_id TEXT NOT NULL,
  node_id TEXT NOT NULL,
  duration_id TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ends_at INTEGER NOT NULL,
  status TEXT NOT NULL,
  result TEXT,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS player_quests (
  player_id TEXT NOT NULL,
  quest_id TEXT NOT NULL,
  status TEXT NOT NULL,
  progress TEXT NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (player_id, quest_id)
);

CREATE TABLE IF NOT EXISTS market_orders (
  id TEXT PRIMARY KEY,
  seller_id TEXT NOT NULL,
  buyer_id TEXT,
  snapshot TEXT NOT NULL,
  price INTEGER NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_market_status ON market_orders(status);

CREATE TABLE IF NOT EXISTS guilds (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  leader_id TEXT NOT NULL,
  level INTEGER NOT NULL,
  exp INTEGER NOT NULL,
  notice TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS guild_members (
  guild_id TEXT NOT NULL,
  player_id TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL,
  joined_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS mail (
  id TEXT PRIMARY KEY,
  player_id TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  attachments TEXT,
  is_read INTEGER NOT NULL DEFAULT 0,
  claimed INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS friends (
  id TEXT PRIMARY KEY,
  from_id TEXT NOT NULL,
  to_id TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chat_messages (
  id TEXT PRIMARY KEY,
  channel TEXT NOT NULL,
  player_id TEXT NOT NULL,
  player_name TEXT NOT NULL,
  text TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS world_state (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`
