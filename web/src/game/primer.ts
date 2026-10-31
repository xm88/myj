/** 进镇、接兽、首战。只看任务和灵兽进度，不改服务器上的主线。 */

export type PrimerStep = 'town' | 'beast' | 'fight' | 'done'

export interface PrimerQuest {
  id?: string
  objectives?: { current: number; need: number }[]
}

export interface PrimerPet {
  role?: string
}

/** 跳过、还没进档、或三步都做过时不再打扰。 */
export function primerStep(input: {
  ready: boolean
  skipped: boolean
  fought: boolean
  quests: PrimerQuest[]
  pets: PrimerPet[]
}): PrimerStep {
  if (!input.ready || input.skipped) return 'done'
  const greet = input.quests.find((quest) => quest.id === 'q_main_01')
  const greeting = greet?.objectives?.some((goal) => goal.current < goal.need)
  if (greeting) return 'town'
  if (!input.pets.some((pet) => pet.role === 'beast')) return 'beast'
  if (!input.fought) return 'fight'
  return 'done'
}
