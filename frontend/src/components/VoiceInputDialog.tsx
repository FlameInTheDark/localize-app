import { useEffect, useMemo, useRef, useState } from "react";
import { FileAudio, Loader2, Mic, Pause, Play, Square, Upload, X } from "lucide-react";
import { beginAudioCapture, toBase64, wavURL, type AudioCapture } from "@/lib/audioCapture";
import { api } from "@/lib/bridge";

type Mode = "choose" | "recording" | "review" | "file" | "transcribing";
const maxRecordingSeconds = 15 * 60;

export function VoiceInputDialog({ microphoneId, language, onClose, onInsert }: { microphoneId?: string; language?: string; onClose: () => void; onInsert: (text: string) => void }) {
  const [mode, setMode] = useState<Mode>("choose");
  const [error, setError] = useState("");
  const [seconds, setSeconds] = useState(0);
  const [audioURL, setAudioURL] = useState("");
  const [recording, setRecording] = useState<Uint8Array | null>(null);
  const [filePath, setFilePath] = useState("");
  const [fileName, setFileName] = useState("");
  const [analyser, setAnalyser] = useState<AnalyserNode | null>(null);
  const capture = useRef<AudioCapture | null>(null);
  const timer = useRef<number | null>(null);
  const audioURLRef = useRef("");

  const stopTimer = () => { if (timer.current !== null) { window.clearInterval(timer.current); timer.current = null; } };
  const cleanupURL = () => { if (audioURLRef.current) URL.revokeObjectURL(audioURLRef.current); audioURLRef.current = ""; };
  const close = () => { stopTimer(); setAnalyser(null); void capture.current?.stop(); capture.current = null; cleanupURL(); onClose(); };
  useEffect(() => () => { stopTimer(); void capture.current?.stop(); cleanupURL(); }, []);

  const startRecord = async () => {
    setError("");
    try {
      const active = await beginAudioCapture(microphoneId, () => undefined);
      capture.current = active;
      setSeconds(0);
      setAnalyser(active.analyser);
      setMode("recording");
      timer.current = window.setInterval(() => setSeconds((value) => {
        if (value + 1 >= maxRecordingSeconds) void stopRecord();
        return value + 1;
      }), 1000);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not start microphone recording");
    }
  };

  const stopRecord = async () => {
    const active = capture.current;
    if (!active) return;
    stopTimer();
    capture.current = null;
    setAnalyser(null);
    const wav = await active.stop();
    if (!wav) {
      setError("No audio was captured. Check your microphone and try again.");
      setMode("choose");
      return;
    }
    cleanupURL();
    setRecording(wav);
    const nextURL = wavURL(wav);
    audioURLRef.current = nextURL;
    setAudioURL(nextURL);
    setMode("review");
  };

  const chooseFile = async () => {
    setError("");
    try {
      const file = await api.pickFile("audio");
      if (!file?.path) return;
      setFilePath(file.path);
      setFileName(file.name);
      setMode("file");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not choose audio file");
    }
  };

  const transcribe = async () => {
    setMode("transcribing");
    setError("");
    try {
      const result = recording
        ? await api.transcribeCapturedAudio({ wavBase64: toBase64(recording), language })
        : await api.transcribeAudio({ path: filePath, language });
      if (!result.text.trim()) {
        setError("No speech was detected. Record a short spoken phrase and try again.");
        setMode(recording ? "review" : "file");
        return;
      }
      onInsert(result.text);
      close();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Transcription failed");
      setMode(recording ? "review" : "file");
    }
  };

  const discard = () => {
    cleanupURL();
    setAudioURL("");
    setRecording(null);
    setFilePath("");
    setFileName("");
    setSeconds(0);
    setMode("choose");
  };

  return <div className="voice-dialog-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && mode !== "recording" && mode !== "transcribing") close(); }}>
    <section className="voice-dialog" role="dialog" aria-modal="true" aria-labelledby="voice-dialog-title">
      <header className="voice-dialog-header">
        <div><p className="text-xs font-medium uppercase tracking-[.14em] text-muted-foreground">Whisper.cpp</p><h2 id="voice-dialog-title" className="mt-1 text-lg font-semibold">Voice input</h2></div>
        <button onClick={close} disabled={mode === "transcribing"} className="app-button app-button-outline !min-h-0 !p-2" aria-label="Close voice input"><X className="size-4" /></button>
      </header>
      {error && <p className="mt-4 rounded-md bg-red-500/10 px-3 py-2 text-sm text-destructive">{error}</p>}
      {mode === "choose" && <div className="mt-5 grid gap-3 sm:grid-cols-2">
        <button onClick={() => void startRecord()} className="voice-source"><Mic className="size-6" /><span><b>Record microphone</b><small>Capture speech and review it before transcription.</small></span></button>
        <button onClick={() => void chooseFile()} className="voice-source"><Upload className="size-6" /><span><b>Choose audio file</b><small>WAV, MP3, FLAC, or OGG from your computer.</small></span></button>
      </div>}
      {mode === "recording" && <div className="mt-5">
        <div className="voice-wave-shell"><LiveWave analyser={analyser} /><div className="voice-wave-meta"><span><i />Recording</span><span className="font-mono tabular-nums">{formatTimer(seconds)}</span></div></div>
        <p className="mt-2 text-center text-xs text-muted-foreground">Listening locally · maximum 15 minutes</p>
        <div className="mt-5 flex justify-center"><button onClick={() => void stopRecord()} className="voice-record-stop"><Square className="size-5 fill-current" />Stop recording</button></div>
      </div>}
      {(mode === "review" || mode === "file") && <div className="mt-5">
        <div className="voice-review">
          {mode === "review" && recording ? <><div className="mb-3"><p className="text-sm font-medium">Recording ready</p><p className="mt-1 text-xs text-muted-foreground">WAV · kept only until transcription completes.</p></div><VoicePlayback src={audioURL} recording={recording} duration={seconds} /></> : <div className="flex items-center gap-3"><FileAudio className="size-6 text-muted-foreground" /><div className="min-w-0"><p className="truncate text-sm font-medium">{fileName}</p><p className="mt-1 text-xs text-muted-foreground">Selected audio file</p></div></div>}
        </div>
        <div className="mt-4 flex justify-between gap-2"><button onClick={discard} className="app-button app-button-outline">Discard</button><button onClick={() => void transcribe()} className="app-button app-button-primary"><Play className="size-4" />Transcribe into text</button></div>
      </div>}
      {mode === "transcribing" && <div className="flex min-h-48 flex-col items-center justify-center"><Loader2 className="size-7 animate-spin" /><p className="mt-3 text-sm font-medium">Transcribing locally…</p><p className="mt-1 text-xs text-muted-foreground">Whisper.cpp is processing your audio on this device.</p></div>}
    </section>
  </div>;
}

function LiveWave({ analyser }: { analyser: AnalyserNode | null }) {
  const canvas = useRef<HTMLCanvasElement | null>(null);
  useEffect(() => {
    const element = canvas.current;
    if (!element || !analyser) return;
    const context = element.getContext("2d");
    if (!context) return;
    const samples = new Uint8Array(analyser.fftSize);
    const style = getComputedStyle(element);
    const active = style.getPropertyValue("--voice-wave-active").trim() || style.color;
    const quiet = style.getPropertyValue("--voice-wave-quiet").trim() || "rgba(255,255,255,.16)";
    let frame = 0;
    const draw = () => {
      analyser.getByteTimeDomainData(samples);
      drawWaveBars(context, element.width, element.height, samples, active, quiet);
      frame = requestAnimationFrame(draw);
    };
    draw();
    return () => window.cancelAnimationFrame(frame);
  }, [analyser]);
  return <canvas ref={canvas} className="voice-wave" width="960" height="144" aria-label="Live microphone waveform" />;
}

function VoicePlayback({ src, recording, duration }: { src: string; recording: Uint8Array; duration: number }) {
  const audio = useRef<HTMLAudioElement | null>(null);
  const canvas = useRef<HTMLCanvasElement | null>(null);
  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const currentTimeRef = useRef(0);
  const levels = useMemo(() => recordedWaveLevels(recording, 100), [recording]);
  useEffect(() => {
    const element = canvas.current;
    if (!element) return;
    const context = element.getContext("2d");
    if (!context) return;
    const style = getComputedStyle(element);
    const active = style.getPropertyValue("--voice-wave-active").trim() || style.color;
    const quiet = style.getPropertyValue("--voice-wave-quiet").trim() || "rgba(255,255,255,.16)";
    let frame = 0;
    const draw = () => {
      const position = audio.current?.currentTime ?? currentTimeRef.current;
      drawRecordedBars(context, element.width, element.height, levels, position / Math.max(duration, 1), active, quiet);
      if (playing) frame = requestAnimationFrame(draw);
    };
    draw();
    return () => window.cancelAnimationFrame(frame);
  }, [duration, levels, playing]);
  const toggle = async () => {
    const element = audio.current;
    if (!element) return;
    if (element.paused) { await element.play(); } else { element.pause(); }
  };
  return <div className="voice-playback">
    <audio ref={audio} src={src} onPlay={() => setPlaying(true)} onPause={() => setPlaying(false)} onEnded={() => { currentTimeRef.current = 0; setPlaying(false); setCurrentTime(0); }} onTimeUpdate={(event) => { currentTimeRef.current = event.currentTarget.currentTime; setCurrentTime(event.currentTarget.currentTime); }} />
    <button onClick={() => void toggle()} className="voice-playback-button" aria-label={playing ? "Pause recording" : "Play recording"}>{playing ? <Pause className="size-4 fill-current" /> : <Play className="size-4 fill-current" />}</button>
    <canvas ref={canvas} className="voice-playback-wave" width="720" height="72" aria-label="Recording waveform" />
    <span className="voice-playback-time">{formatTimer(Math.floor(currentTime || duration))}</span>
  </div>;
}

function drawWaveBars(context: CanvasRenderingContext2D, width: number, height: number, samples: Uint8Array, active: string, quiet: string) {
  context.clearRect(0, 0, width, height);
  const barCount = 104;
  const gap = 4;
  const barWidth = (width - (barCount - 1) * gap) / barCount;
  const rawLevels = new Array<number>(barCount);
  for (let bar = 0; bar < barCount; bar += 1) {
    const start = Math.floor(bar * samples.length / barCount);
    const end = Math.max(start + 1, Math.floor((bar + 1) * samples.length / barCount));
    rawLevels[bar] = rmsByteRange(samples, start, end);
  }
  const levels = gateWaveLevels(rawLevels, .012, .035);
  for (let bar = 0; bar < barCount; bar += 1) {
    const barHeight = Math.max(6, Math.min(height * .82, 6 + levels[bar] * height * .78));
    context.fillStyle = levels[bar] > .01 ? active : quiet;
    fillRoundedBar(context, bar * (barWidth + gap), (height - barHeight) / 2, barWidth, barHeight);
  }
}

function drawRecordedBars(context: CanvasRenderingContext2D, width: number, height: number, levels: number[], progress: number, active: string, quiet: string) {
  context.clearRect(0, 0, width, height);
  const barCount = 100;
  const gap = 4;
  const barWidth = (width - (barCount - 1) * gap) / barCount;
  context.fillStyle = quiet;
  for (let bar = 0; bar < barCount; bar += 1) {
    const barHeight = Math.max(5, Math.min(height * .8, 5 + levels[bar] * height * .8));
    fillRoundedBar(context, bar * (barWidth + gap), (height - barHeight) / 2, barWidth, barHeight);
  }
  context.save();
  context.beginPath();
  context.rect(0, 0, width * Math.max(0, Math.min(1, progress)), height);
  context.clip();
  context.fillStyle = active;
  for (let bar = 0; bar < barCount; bar += 1) {
    const barHeight = Math.max(5, Math.min(height * .8, 5 + levels[bar] * height * .8));
    fillRoundedBar(context, bar * (barWidth + gap), (height - barHeight) / 2, barWidth, barHeight);
  }
  context.restore();
}

function recordedWaveLevels(recording: Uint8Array, barCount: number) {
  const audioBytes = Math.max(0, recording.byteLength - 44);
  const levels = new Array<number>(barCount);
  for (let bar = 0; bar < barCount; bar += 1) {
    const start = 44 + Math.floor(bar * audioBytes / barCount);
    const end = Math.min(recording.byteLength - 1, 44 + Math.floor((bar + 1) * audioBytes / barCount));
    levels[bar] = rmsPCMRange(recording, start, end);
  }
  return gateWaveLevels(levels, .009, .025);
}

function gateWaveLevels(levels: number[], minimumGate: number, minimumRange: number) {
  const sorted = [...levels].sort((left, right) => left - right);
  const noiseFloor = sorted[Math.floor(sorted.length * .2)] ?? 0;
  const loudLevel = sorted[Math.floor(sorted.length * .9)] ?? 0;
  const gate = Math.max(minimumGate, noiseFloor * 1.65);
  const range = Math.max(minimumRange, loudLevel - gate);
  return levels.map((level) => Math.sqrt(Math.max(0, Math.min(1, (level - gate) / range))));
}

function rmsByteRange(samples: Uint8Array, start: number, end: number) {
  let sum = 0;
  for (let index = start; index < end; index += 1) { const value = (samples[index] - 128) / 128; sum += value * value; }
  return Math.sqrt(sum / Math.max(1, end - start));
}

function rmsPCMRange(samples: Uint8Array, start: number, end: number) {
  const alignedStart = start + start % 2;
  const alignedEnd = end - end % 2;
  const availableSamples = Math.floor((alignedEnd - alignedStart) / 2);
  if (availableSamples < 1) return 0;
  const windows = Math.min(3, Math.max(1, Math.floor(availableSamples / 24)));
  const samplesPerWindow = Math.min(96, Math.max(1, Math.floor(availableSamples / windows)));
  let sum = 0;
  let count = 0;
  for (let window = 0; window < windows; window += 1) {
    const center = Math.floor((window + .5) * availableSamples / windows);
    const firstSample = Math.max(0, Math.min(availableSamples - samplesPerWindow, center - Math.floor(samplesPerWindow / 2)));
    for (let sample = 0; sample < samplesPerWindow; sample += 1) {
      const offset = alignedStart + (firstSample + sample) * 2;
      const value = ((samples[offset] | (samples[offset + 1] << 8)) << 16 >> 16) / 32768;
      sum += value * value;
      count += 1;
    }
  }
  return Math.sqrt(sum / Math.max(1, count));
}

function fillRoundedBar(context: CanvasRenderingContext2D, x: number, y: number, width: number, height: number) {
  const radius = Math.min(width / 2, height / 2);
  context.beginPath();
  context.roundRect(x, y, width, height, radius);
  context.fill();
}

function formatTimer(seconds: number) { const minutes = Math.floor(seconds / 60).toString().padStart(2, "0"); const remainder = (seconds % 60).toString().padStart(2, "0"); return `${minutes}:${remainder}`; }
