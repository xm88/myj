/** 场景怪物聚合：同模板只显示一个标记，操作指向距离角色最近的实例。 */

/** 场景聚合所需的怪物快照字段，包含位置与契约状态。 */
export interface CrowdMob {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 实例关联的配置模板标识。 */
  templateId: string
  /** 世界横坐标，单位与地图尺寸保持一致。 */
  x: number
  /** 世界纵坐标，单位与地图尺寸保持一致。 */
  y: number
  /** 怪物类别或公会职位，具体含义由所属结构决定。 */
  role?: string
  /** 该怪物模板是否支持契约，具体条件由服务器继续校验。 */
  capturable?: boolean
  /** 当前生命值或模板生命属性。 */
  hp?: number
  /** 最终面板的生命上限。 */
  maxHp?: number
}

/** 同模板怪物的聚合标记，使用最近实例的位置并附带场内数量。 */
export interface Pack extends CrowdMob {
  /** 堆叠数量或任务目标所需次数。 */
  count: number
}

/** 同一种怪物收成一个点，战斗打离你最近的那只。 */
export function nearestPacks(monsters: CrowdMob[], self: { x: number; y: number }): Pack[] {
  const groups = new Map<string, Pack & { distance: number }>()
  for (const mob of monsters) {
    const distance = Math.hypot(mob.x - self.x, mob.y - self.y)
    const current = groups.get(mob.templateId)
    if (!current || distance < current.distance) {
      groups.set(mob.templateId, {
        id: mob.id,
        name: mob.name,
        templateId: mob.templateId,
        x: mob.x,
        y: mob.y,
        role: mob.role,
        capturable: Boolean(current?.capturable || mob.capturable),
        hp: mob.hp,
        maxHp: mob.maxHp,
        count: (current?.count ?? 0) + 1,
        distance,
      })
    } else {
      current.count += 1
      if (mob.capturable) current.capturable = true
    }
  }
  return [...groups.values()].sort((a, b) => a.distance - b.distance)
}
