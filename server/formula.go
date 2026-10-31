// 战斗与成长公式：伤害、经验、种族属性、品质和真实时间映射；回归基准固定这些规则。

package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

// weaponLabels 武器协议标识与中文名称的对应关系。
var weaponLabels = M{"sword_shield": "剑盾", "greatsword": "巨剑", "staff": "法杖", "bow": "弓"}

// periodLabels 游戏时段与中文名称的对应关系。
var periodLabels = M{"morning": "早晨", "day": "白天", "dusk": "黄昏", "night": "夜晚", "late": "深夜"}

// qualities 品质按强度从低到高排列，任务比较使用同一顺序。
var qualities = []string{"white", "green", "blue", "purple", "gold"}

// qualityMul 不同装备品质叠加基础属性时使用的倍率。
var qualityMul = map[string]float64{"white": 1, "green": 1.12, "blue": 1.28, "purple": 1.5, "gold": 1.75}

// qualityAffixes 每个装备品质允许生成的随机词条数量。
var qualityAffixes = map[string]int{"white": 0, "green": 1, "blue": 2, "purple": 3, "gold": 3}

// qualityNames 装备品质在制造和淬炼提示里的中文。
var qualityNames = map[string]string{"white": "普通", "green": "优秀", "blue": "精良", "purple": "史诗", "gold": "传说"}

// qualityName 把品质标识写成中文。不认识的按普通。
func qualityName(q string) string {
	if name := qualityNames[q]; name != "" {
		return name
	}
	return qualityNames["white"]
}

// attrKeys 潜力点的固定字段顺序，加点校验和界面展示共用。
var attrKeys = []string{"vit", "mag", "str", "agi", "end"}

// attrNames 五项潜力点对应的中文名称。
var attrNames = M{"vit": "体力", "mag": "魔力", "str": "力量", "agi": "敏捷", "end": "耐力"}

// races 三族每级成长与面板换算参数，和迁移前数值保持一致。
var races = map[string]M{
	"human":    {"id": "human", "name": "人族", "blurb": "五项一起长，没有短板。", "growth": M{"vit": 1, "mag": 1, "str": 1, "end": 1, "agi": 1}, "rate": M{"vitHp": 5, "magMp": 3, "strAtk": .66, "endDef": 1.5}},
	"demon":    {"id": "demon", "name": "魔族", "blurb": "体力和力量长得快，耐力几乎不长。", "growth": M{"vit": 2, "mag": 0, "str": 2, "end": 0, "agi": 1}, "rate": M{"vitHp": 6, "magMp": 2.5, "strAtk": .76, "endDef": 1.4}},
	"immortal": {"id": "immortal", "name": "仙族", "blurb": "魔力和耐力长得快，体力几乎不长。", "growth": M{"vit": 0, "mag": 2, "str": 0, "end": 2, "agi": 1}, "rate": M{"vitHp": 4.5, "magMp": 3.5, "strAtk": .57, "endDef": 1.6}},
}

// raceID 校验种族 ID，旧存档或无效值按人族处理。
func raceID(s string) string {
	if races[s] == nil {
		return "human"
	}
	return s
}

// emptyAttrs 生成五项潜力点均为零的属性对象。
func emptyAttrs() M { return M{"vit": 0, "mag": 0, "str": 0, "agi": 0, "end": 0} }

// parseAttrs 读取存档潜力点，将缺失、负值或异常字段视为未分配。
func parseAttrs(s string) M {
	m := obj(parse(s))
	out := emptyAttrs()
	for _, k := range attrKeys {
		out[k] = math.Max(0, math.Floor(num(m[k])))
	}
	return out
}

// totalAttrs 叠加种族每级自动成长与玩家已经分配的潜力点。
func totalAttrs(level float64, spent M, race string) M {
	out := clone(spent)
	growth := obj(races[raceID(race)]["growth"])
	for _, k := range attrKeys {
		out[k] = num(out[k]) + num(growth[k])*math.Max(0, math.Floor(level))
	}
	return out
}

// spentAttrs 汇总五项已分配潜力，用于剩余点数和加点上限校验。
func spentAttrs(m M) float64 {
	n := 0.
	for _, k := range attrKeys {
		n += num(m[k])
	}
	return n
}

// attrBonus 按种族比例把属性换算为生命、法力、伤害、防御、灵力和速度。
func attrBonus(stats, attrs M, race string) {
	r := obj(races[raceID(race)]["rate"])
	v, m, s, e, g := num(attrs["vit"]), num(attrs["mag"]), num(attrs["str"]), num(attrs["end"]), num(attrs["agi"])
	stats["maxHp"] = num(stats["maxHp"]) + v*num(r["vitHp"])
	stats["maxMp"] = num(stats["maxMp"]) + m*num(r["magMp"])
	stats["atk"] = num(stats["atk"]) + s*num(r["strAtk"])
	stats["def"] = num(stats["def"]) + e*num(r["endDef"])
	stats["spell"] = num(stats["spell"]) + v*.3 + m*.7 + s*.4 + e*.2
	speed := (v+s+e)*.1 + g*.7
	stats["attackInterval"] = math.Max(650, num(stats["attackInterval"])-speed*4)
	stats["speed"] = num(stats["speed"]) + speed*.8
}

// expNext 计算角色升到下一级所需经验，采用随等级增长的指数曲线。
func expNext(level float64) float64 { return math.Round(100 * math.Pow(math.Max(1, level), 1.35)) }

// trackNext 根据武器或生活技能类型计算下一熟练等级所需经验。
func trackNext(level float64, weapon bool) float64 {
	if weapon {
		return 40 + level*30
	}
	return 30 + level*25
}

// dayPeriod 按 UTC 加八小时映射游戏时段，全服共用同一时段判定。
func dayPeriod(t time.Time) string {
	hour := (t.UTC().Hour() + 8) % 24
	switch {
	case hour >= 5 && hour < 8:
		return "morning"
	case hour >= 8 && hour < 17:
		return "day"
	case hour >= 17 && hour < 19:
		return "dusk"
	case hour >= 19 && hour < 23:
		return "night"
	}
	return "late"
}

// damage 依次结算技能倍率、防御减伤、元素、暴击和易伤；伤害至少为一点。
func damage(attack, coefficient, def, bonus, resist, critRate, critDmg, taken float64, force bool, rng func() float64) (float64, bool) {
	crit := force || rng() < critRate
	mul := 1.
	if crit {
		mul = critDmg
	}
	def = math.Max(0, def)
	return math.Max(1, math.Round(attack*coefficient*(1-def/(def+500))*math.Max(.2, 1+bonus-resist)*mul*taken)), crit
}

// growthLine 把种族自动成长转换为中文展示文本，只显示有增长的属性。
func growthLine(race string) string {
	s := ""
	g := obj(races[raceID(race)]["growth"])
	for _, k := range attrKeys {
		if num(g[k]) <= 0 {
			continue
		}
		if s != "" {
			s += "，"
		}
		s += fmt.Sprintf("%s +%g", str(attrNames[k]), num(g[k]))
	}
	return s
}

// raceList 输出创建角色界面需要的种族名称、简介和成长说明。
func raceList() []M {
	out := []M{}
	for _, id := range []string{"human", "demon", "immortal"} {
		r := races[id]
		out = append(out, M{"id": id, "name": r["name"], "blurb": r["blurb"], "growth": growthLine(id)})
	}
	return out
}

// weaponList 输出创建角色界面可选武器及其玩法简介。
func weaponList() []M {
	out := []M{}
	blurbs := []string{"扛得住，适合和灵伴慢慢磨。", "慢，但每一刀都很重。", "元素和治疗都在这根杖上。", "站得远，暴击更高。"}
	for i, id := range []string{"sword_shield", "greatsword", "staff", "bow"} {
		out = append(out, M{"id": id, "name": weaponLabels[id], "blurb": blurbs[i]})
	}
	return out
}

// qualityRank 将装备品质转换为可比较的等级，用于任务品质门槛。
func qualityRank(s string) int {
	for i, q := range qualities {
		if q == s {
			return i
		}
	}
	return -1
}

// randInt 在给定范围内生成随机整数，用于金币等奖励的数值浮动。
func randInt(min, max float64) float64 { return math.Floor(min + rand.Float64()*(max-min+1)) }
