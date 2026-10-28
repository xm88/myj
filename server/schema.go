// SQLite 存档结构：账号、角色、熟练、物品、灵伴、派遣、任务、交易、公会、邮件与世界状态。

package main

// schema 兼容旧 SQLite 存档的建表语句；金额、物品与任务分表持久化。
const schema = `
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

CREATE INDEX IF NOT EXISTS idx_dispatch_player ON pet_dispatch(player_id, status);
CREATE INDEX IF NOT EXISTS idx_mail_player ON mail(player_id, created_at);
CREATE INDEX IF NOT EXISTS idx_friends_pair ON friends(from_id, to_id);
CREATE INDEX IF NOT EXISTS idx_chat_channel ON chat_messages(channel, created_at);
`
