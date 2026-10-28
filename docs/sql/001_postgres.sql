-- 浮岛旅团 V0.1 的 PostgreSQL 目标表。
-- 当前进程用 SQLite，列名和这里一致。迁过去时把 extra、snapshot、attachments、progress 改成 JSONB 即可。
-- 攻击和防御由等级、装备、灵伴和加点现算。加点本身存在 players.attributes。

-- 账号及密码摘要；邮箱唯一。
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  is_guest INTEGER NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL
);

-- 角色主存档，关联账号，保存成长、位置和潜力点。
CREATE TABLE players (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL UNIQUE REFERENCES users(id),
  name TEXT NOT NULL UNIQUE,
  level INTEGER NOT NULL,
  exp INTEGER NOT NULL,
  map_id TEXT NOT NULL,
  x DOUBLE PRECISION NOT NULL,
  y DOUBLE PRECISION NOT NULL,
  hp DOUBLE PRECISION NOT NULL,
  mp DOUBLE PRECISION NOT NULL,
  gold INTEGER NOT NULL,
  weapon_type TEXT NOT NULL,
  skill_loadout JSONB NOT NULL,
  attributes JSONB NOT NULL DEFAULT '{}',
  race TEXT NOT NULL DEFAULT 'human',
  flags JSONB NOT NULL DEFAULT '{}',
  buffs JSONB NOT NULL DEFAULT '[]',
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);

-- 每个角色的武器与生活熟练；角色加熟练类型组成唯一键。
CREATE TABLE player_tracks (
  player_id TEXT NOT NULL REFERENCES players(id),
  track TEXT NOT NULL,
  level INTEGER NOT NULL,
  exp INTEGER NOT NULL,
  PRIMARY KEY (player_id, track)
);

-- 物品实例及其背包、仓库和装备位置，词条放在 extra。
CREATE TABLE player_items (
  id TEXT PRIMARY KEY,
  player_id TEXT NOT NULL REFERENCES players(id),
  template_id TEXT NOT NULL,
  count INTEGER NOT NULL,
  location TEXT NOT NULL,
  extra JSONB,
  created_at BIGINT NOT NULL
);
CREATE INDEX idx_items_player ON player_items(player_id, location);

-- 角色拥有的灵伴实例及主战和成长状态。
CREATE TABLE player_pets (
  id TEXT PRIMARY KEY,
  player_id TEXT NOT NULL REFERENCES players(id),
  template_id TEXT NOT NULL,
  level INTEGER NOT NULL,
  exp INTEGER NOT NULL,
  active INTEGER NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL
);

-- 离线派遣起止时间、领取状态与结算结果。
CREATE TABLE pet_dispatch (
  id TEXT PRIMARY KEY,
  player_id TEXT NOT NULL REFERENCES players(id),
  player_pet_id TEXT NOT NULL REFERENCES player_pets(id),
  node_id TEXT NOT NULL,
  duration_id TEXT NOT NULL,
  started_at BIGINT NOT NULL,
  ends_at BIGINT NOT NULL,
  status TEXT NOT NULL,
  result JSONB,
  created_at BIGINT NOT NULL
);

-- 服务器任务进度与交付状态；同一角色同一任务只保存一条。
CREATE TABLE player_quests (
  player_id TEXT NOT NULL REFERENCES players(id),
  quest_id TEXT NOT NULL,
  status TEXT NOT NULL,
  progress JSONB NOT NULL,
  updated_at BIGINT NOT NULL,
  PRIMARY KEY (player_id, quest_id)
);

-- 交易订单的物品快照、价格、买卖双方与状态。
CREATE TABLE market_orders (
  id TEXT PRIMARY KEY,
  seller_id TEXT NOT NULL REFERENCES players(id),
  buyer_id TEXT REFERENCES players(id),
  snapshot JSONB NOT NULL,
  price INTEGER NOT NULL,
  status TEXT NOT NULL,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);
CREATE INDEX idx_market_status ON market_orders(status);

-- 公会名称、会长、等级、经验与公告。
CREATE TABLE guilds (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  leader_id TEXT NOT NULL REFERENCES players(id),
  level INTEGER NOT NULL,
  exp INTEGER NOT NULL,
  notice TEXT NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL
);

-- 公会成员和职务；每个角色至多属于一个公会。
CREATE TABLE guild_members (
  guild_id TEXT NOT NULL REFERENCES guilds(id),
  player_id TEXT NOT NULL UNIQUE REFERENCES players(id),
  role TEXT NOT NULL,
  joined_at BIGINT NOT NULL
);

-- 邮件正文、奖励附件与一次性领取状态。
CREATE TABLE mail (
  id TEXT PRIMARY KEY,
  player_id TEXT NOT NULL REFERENCES players(id),
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  attachments JSONB,
  is_read INTEGER NOT NULL DEFAULT 0,
  claimed INTEGER NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL
);

-- 好友申请的双方角色与接受状态。
CREATE TABLE friends (
  id TEXT PRIMARY KEY,
  from_id TEXT NOT NULL REFERENCES players(id),
  to_id TEXT NOT NULL REFERENCES players(id),
  status TEXT NOT NULL,
  created_at BIGINT NOT NULL
);

-- 按世界、地图或公会频道存储的聊天历史。
CREATE TABLE chat_messages (
  id TEXT PRIMARY KEY,
  channel TEXT NOT NULL,
  player_id TEXT NOT NULL REFERENCES players(id),
  player_name TEXT NOT NULL,
  text TEXT NOT NULL,
  created_at BIGINT NOT NULL
);

-- 服务器重启后保留的全服天气与新闻键值记录。
CREATE TABLE world_state (
  key TEXT PRIMARY KEY,
  value JSONB NOT NULL
);
