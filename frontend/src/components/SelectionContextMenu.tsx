import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Copy } from "lucide-react";
import { ClipboardSetText } from "../../wailsjs/runtime/runtime";

type TextControl = HTMLInputElement | HTMLTextAreaElement;
type SelectionMenu = {
  text: string;
  control: TextControl | null;
  x: number;
  y: number;
};

const viewportMargin = 8;

export function SelectionContextMenu() {
  const [menu, setMenu] = useState<SelectionMenu | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);

  useLayoutEffect(() => {
    if (!menu || !menuRef.current) return;
    const bounds = menuRef.current.getBoundingClientRect();
    const x = Math.max(viewportMargin, Math.min(menu.x, window.innerWidth - bounds.width - viewportMargin));
    const y = Math.max(viewportMargin, Math.min(menu.y, window.innerHeight - bounds.height - viewportMargin));
    if (x !== menu.x || y !== menu.y) setMenu((current) => current ? { ...current, x, y } : null);
  }, [menu]);

  useEffect(() => {
    const close = () => setMenu(null);
    const closeOnPointerDown = (event: PointerEvent) => {
      if (menuRef.current?.contains(event.target as Node)) return;
      close();
    };
    const closeOnKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    const onContextMenu = (event: MouseEvent) => {
      if (menuRef.current?.contains(event.target as Node)) {
        event.preventDefault();
        return;
      }
      event.preventDefault();
      const selection = readSelection(event.target);
      if (!selection) {
        close();
        return;
      }
      setMenu({ ...selection, x: event.clientX, y: event.clientY });
    };

    window.addEventListener("contextmenu", onContextMenu);
    window.addEventListener("pointerdown", closeOnPointerDown);
    window.addEventListener("keydown", closeOnKeyDown);
    window.addEventListener("blur", close);
    window.addEventListener("resize", close);
    window.addEventListener("scroll", close, true);
    return () => {
      window.removeEventListener("contextmenu", onContextMenu);
      window.removeEventListener("pointerdown", closeOnPointerDown);
      window.removeEventListener("keydown", closeOnKeyDown);
      window.removeEventListener("blur", close);
      window.removeEventListener("resize", close);
      window.removeEventListener("scroll", close, true);
    };
  }, []);

  if (!menu) return null;
  const copy = (text: string) => {
    void copyText(text);
    setMenu(null);
  };
  const selectAll = () => {
    menu.control?.focus();
    menu.control?.select();
    setMenu(null);
  };

  return createPortal(
    <div ref={menuRef} className="selection-context-menu" role="menu" style={{ left: menu.x, top: menu.y }} onContextMenu={(event) => event.preventDefault()}>
      <button type="button" role="menuitem" className="selection-context-menu-item" onClick={() => copy(menu.text)}><Copy className="size-3.5" />Copy</button>
      {menu.control && <>
        <div className="selection-context-menu-divider" />
        <button type="button" role="menuitem" className="selection-context-menu-item" onClick={() => copy(menu.control?.value ?? "")}>Copy all</button>
        <button type="button" role="menuitem" className="selection-context-menu-item" onClick={selectAll}>Select all</button>
      </>}
    </div>,
    document.body,
  );
}

function readSelection(target: EventTarget | null): Pick<SelectionMenu, "text" | "control"> | null {
  if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) {
    const start = target.selectionStart;
    const end = target.selectionEnd;
    if (start !== null && end !== null && end > start) return { text: target.value.slice(start, end), control: target };
  }
  const text = window.getSelection()?.toString() ?? "";
  return text.trim() ? { text, control: null } : null;
}

async function copyText(text: string) {
  try {
    await ClipboardSetText(text);
  } catch {
    await navigator.clipboard?.writeText(text);
  }
}
