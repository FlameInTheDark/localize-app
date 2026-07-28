import { useEffect, useState } from "react";
import { Maximize2, Minimize2, Square, X } from "lucide-react";
import { Quit, WindowIsMaximised, WindowMinimise, WindowToggleMaximise } from "../../wailsjs/runtime/runtime";

export function WindowControls() {
  const [maximised, setMaximised] = useState(false);
  const syncState = () => void WindowIsMaximised().then(setMaximised).catch(() => setMaximised(false));

  useEffect(() => { syncState(); }, []);

  const toggle = () => {
    WindowToggleMaximise();
    window.setTimeout(syncState, 80);
  };

  return <div className="window-controls">
    <button type="button" className="window-control" onClick={WindowMinimise} aria-label="Minimise"><Minimize2 className="size-3.5" /></button>
    <button type="button" className="window-control" onClick={toggle} aria-label={maximised ? "Restore window" : "Maximise window"}>{maximised ? <Square className="size-3" /> : <Maximize2 className="size-3.5" />}</button>
    <button type="button" className="window-control window-control-close" onClick={Quit} aria-label="Close Localize"><X className="size-4" /></button>
  </div>;
}
