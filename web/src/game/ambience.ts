/** 场景氛围底噪。没有现成音效，用很轻的噪声和正弦。开关记在本机。 */

let ctx: AudioContext | null = null
let bed: { stop: () => void } | null = null
let signature = ''

function audioContext() {
  const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
  if (!Ctx) return null
  if (!ctx) ctx = new Ctx()
  return ctx
}

function stop() {
  bed?.stop()
  bed = null
}

function noise(context: AudioContext) {
  const buffer = context.createBuffer(1, context.sampleRate * 2, context.sampleRate)
  const data = buffer.getChannelData(0)
  let last = 0
  for (let i = 0; i < data.length; i++) {
    const white = Math.random() * 2 - 1
    last = (last + white * 0.02) / 1.02
    data[i] = last * 3.5
  }
  const source = context.createBufferSource()
  source.buffer = buffer
  source.loop = true
  return source
}

function start(context: AudioContext, weather: string, period: string) {
  const master = context.createGain()
  let gain = 0.03
  let freq = 720
  if (weather === 'storm') {
    gain = 0.048
    freq = 1500
  } else if (weather === 'rain') {
    gain = 0.04
    freq = 1300
  } else if (weather === 'wind') {
    gain = 0.038
    freq = 480
  } else if (weather === 'snow' || weather === 'fog') {
    gain = 0.02
    freq = 380
  } else if (period === 'night' || period === 'late') {
    gain = 0.026
    freq = 280
  }
  master.gain.value = gain
  const filter = context.createBiquadFilter()
  filter.type = 'lowpass'
  filter.frequency.value = freq
  const source = noise(context)
  source.connect(filter)
  filter.connect(master)
  const tone = context.createOscillator()
  tone.type = 'sine'
  tone.frequency.value = period === 'night' || period === 'late' ? 174 : weather === 'storm' ? 92 : 220
  const toneGain = context.createGain()
  toneGain.gain.value = 0.01
  tone.connect(toneGain)
  toneGain.connect(master)
  master.connect(context.destination)
  source.start()
  tone.start()
  return {
    stop() {
      try {
        source.stop()
        tone.stop()
      } catch {
        /* 已经停过。 */
      }
      master.disconnect()
    },
  }
}

/** 按天气和时段铺一层底噪。关掉或环境没变时不再重开。 */
export function syncAmbience(on: boolean, weather: string, period: string) {
  if (!on) {
    stop()
    signature = ''
    return
  }
  const next = `${weather}:${period}`
  if (signature === next && bed) return
  const context = audioContext()
  if (!context) return
  void context.resume()
  stop()
  bed = start(context, weather, period)
  signature = next
}

if (typeof window !== 'undefined') {
  window.addEventListener('pointerdown', () => {
    if (ctx?.state === 'suspended') void ctx.resume()
  })
}
