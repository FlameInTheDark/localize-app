const targetSampleRate = 16_000;

export type AudioChunk = { samples: Int16Array; rms: number };

export class PCMRecorder {
  private readonly chunks: Int16Array[] = [];
  private count = 0;

  append(input: Float32Array, inputRate: number): AudioChunk {
    const length = Math.max(1, Math.floor(input.length * targetSampleRate / inputRate));
    const samples = new Int16Array(length);
    let sum = 0;
    for (let index = 0; index < length; index += 1) {
      const from = Math.min(input.length - 1, Math.floor(index * inputRate / targetSampleRate));
      const value = Math.max(-1, Math.min(1, input[from] ?? 0));
      sum += value * value;
      samples[index] = value < 0 ? value * 0x8000 : value * 0x7fff;
    }
    this.chunks.push(samples); this.count += samples.length;
    return { samples, rms: Math.sqrt(sum / Math.max(length, 1)) };
  }

  get sampleCount() { return this.count; }
  get seconds() { return this.count / targetSampleRate; }
  clear() { this.chunks.length = 0; this.count = 0; }

  takeWAV(): Uint8Array | null {
    if (this.count === 0) return null;
    const wav = createWAV(this.chunks, this.count);
    this.clear();
    return wav;
  }
}

export type AudioCapture = { analyser: AnalyserNode; stop: () => Promise<Uint8Array | null> };

export async function beginAudioCapture(deviceId: string | undefined, onChunk: (chunk: AudioChunk) => void): Promise<AudioCapture> {
  if (!navigator.mediaDevices?.getUserMedia || !window.AudioContext) throw new Error("Audio recording is unavailable in this WebView. Update WebView2 and try again.");
  const constraints: MediaTrackConstraints = deviceId ? { deviceId: { exact: deviceId }, channelCount: 1, echoCancellation: true, noiseSuppression: true } : { channelCount: 1, echoCancellation: true, noiseSuppression: true };
  const stream = await navigator.mediaDevices.getUserMedia({ audio: constraints });
  const context = new AudioContext();
  await context.resume();
  const source = context.createMediaStreamSource(stream);
  const analyser = context.createAnalyser(); analyser.fftSize = 512; analyser.smoothingTimeConstant = 0.82;
  const silence = context.createGain(); silence.gain.value = 0;
  const recorder = new PCMRecorder();
  const moduleURL = URL.createObjectURL(new Blob([workletSource], { type: "text/javascript" }));
  try {
    await context.audioWorklet.addModule(moduleURL);
  } finally { URL.revokeObjectURL(moduleURL); }
  const node = new AudioWorkletNode(context, "localize-pcm");
  let acknowledgeFlush: (() => void) | null = null;
  node.port.onmessage = (event: MessageEvent<ArrayBuffer | { done: true }>) => {
    if (event.data instanceof ArrayBuffer) {
      const input = new Float32Array(event.data);
      onChunk(recorder.append(input, context.sampleRate));
      return;
    }
    acknowledgeFlush?.(); acknowledgeFlush = null;
  };
  source.connect(analyser); source.connect(node); node.connect(silence); silence.connect(context.destination);
  let stopped = false;
  return {
    analyser,
    stop: async () => {
      if (stopped) return null;
      stopped = true;
      await Promise.race([new Promise<void>((resolve) => { acknowledgeFlush = resolve; node.port.postMessage("flush"); }), new Promise<void>((resolve) => window.setTimeout(resolve, 160))]);
      node.port.onmessage = null;
      source.disconnect(); node.disconnect(); silence.disconnect();
      stream.getTracks().forEach((track) => track.stop());
      await context.close();
      return recorder.takeWAV();
    },
  };
}

export function toBase64(data: Uint8Array): string {
  const chunkSize = 0x8000; const chunks: string[] = [];
  for (let index = 0; index < data.length; index += chunkSize) chunks.push(String.fromCharCode(...data.subarray(index, index + chunkSize)));
  return btoa(chunks.join(""));
}

export function wavURL(data: Uint8Array): string { const copy = Uint8Array.from(data); return URL.createObjectURL(new Blob([copy.buffer], { type: "audio/wav" })); }
export function wavFromSamples(chunks: Int16Array[]): Uint8Array | null {
  const count = chunks.reduce((total, chunk) => total + chunk.length, 0);
  return count > 0 ? createWAV(chunks, count) : null;
}

function createWAV(chunks: Int16Array[], sampleCount: number): Uint8Array {
  const bytes = new Uint8Array(44 + sampleCount * 2); const view = new DataView(bytes.buffer);
  writeString(view, 0, "RIFF"); view.setUint32(4, 36 + sampleCount * 2, true); writeString(view, 8, "WAVE"); writeString(view, 12, "fmt ");
  view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, 1, true); view.setUint32(24, targetSampleRate, true); view.setUint32(28, targetSampleRate * 2, true); view.setUint16(32, 2, true); view.setUint16(34, 16, true); writeString(view, 36, "data"); view.setUint32(40, sampleCount * 2, true);
  let offset = 44; for (const chunk of chunks) { for (let index = 0; index < chunk.length; index += 1) { view.setInt16(offset, chunk[index], true); offset += 2; } }
  return bytes;
}

function writeString(view: DataView, offset: number, text: string) { for (let index = 0; index < text.length; index += 1) view.setUint8(offset + index, text.charCodeAt(index)); }

const workletSource = `
class LocalizePCMProcessor extends AudioWorkletProcessor {
  constructor() { super(); this.parts = []; this.length = 0; this.port.onmessage = () => { this.flush(); this.port.postMessage({ done: true }); }; }
  flush() { if (!this.length) return; const output = new Float32Array(this.length); let offset = 0; for (const part of this.parts) { output.set(part, offset); offset += part.length; } this.parts = []; this.length = 0; this.port.postMessage(output.buffer, [output.buffer]); }
  process(inputs) {
    const channel = inputs[0] && inputs[0][0];
    if (!channel) return true;
    const copy = new Float32Array(channel);
    this.parts.push(copy); this.length += copy.length;
    if (this.length >= 2048) this.flush();
    return true;
  }
}
registerProcessor("localize-pcm", LocalizePCMProcessor);`;
