import { ArrowUp } from "lucide-react";
import { useLayoutEffect, useRef } from "react";

import type { Agent } from "@/entities/agent";
import { StopRunButton } from "@/features/stop-run";
import { Button, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui";

const maxLines = 8;
const fallbackLineHeight = 24;

export type ChatComposerProps = {
  value: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
  placeholder: string;
  disabled: boolean;
  busy: boolean;
  switching: boolean;
  agents: Agent[];
  agentId: string;
  onAgentChange: (agentId: string) => void;
  onStop: () => void;
};

export function ChatComposer({ value, onChange, onSubmit, placeholder, disabled, busy, switching, agents, agentId, onAgentChange, onStop }: ChatComposerProps) {
  const boxRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  // The layout switch is written straight to the DOM instead of held in state:
  // it is decided by a measurement that only exists after layout, and a state
  // round-trip would render the box at the wrong width for a frame first.
  useLayoutEffect(() => {
    const box = boxRef.current;
    const input = inputRef.current;
    if (!box || !input) return;

    const fit = () => {
      const style = window.getComputedStyle(input);
      const lineHeight = Number.parseFloat(style.lineHeight) || fallbackLineHeight;
      // scrollHeight counts the input's own padding, so every comparison against
      // a number of lines has to carry it too.
      const padding = (Number.parseFloat(style.paddingTop) || 0) + (Number.parseFloat(style.paddingBottom) || 0);
      const oneLine = lineHeight + padding;
      const maxHeight = lineHeight * maxLines + padding;
      const measure = () => { input.style.height = "auto"; return input.scrollHeight; };

      // Always judge the switch at the one-row width, never at whatever layout
      // is currently on screen. The input is wider once the controls drop to
      // their own row, so a text measured there can report "fits on one line"
      // while it would wrap the moment the box went back to one row — and the
      // box would then flip on every keystroke across a whole range of lengths.
      box.dataset.layout = "inline";
      if (measure() > oneLine + 1 || value.includes("\n")) box.dataset.layout = "stacked";

      // Measure again: the line above changes how wide the input is.
      const content = measure();
      input.style.height = `${Math.min(Math.max(content, oneLine), maxHeight)}px`;
      input.style.overflowY = content > maxHeight ? "auto" : "hidden";
    };

    fit();
    if (typeof ResizeObserver === "undefined") return;
    // A pixel height stays correct only for the width it was measured at, so a
    // draft left in the box would be clipped with no scrollbar after a resize.
    // Only width matters here; reacting to the height fit() itself sets would
    // just feed the observer its own output.
    let width = box.clientWidth;
    const observer = new ResizeObserver(() => {
      if (box.clientWidth === width) return;
      width = box.clientWidth;
      fit();
    });
    observer.observe(box);
    return () => observer.disconnect();
  }, [value]);

  return (
    <div ref={boxRef} data-layout="inline" className="group flex gap-2 rounded-3xl border border-input bg-card p-2 shadow-[0_1px_2px_rgb(23_35_59/0.04)] transition-[border-color,box-shadow] duration-200 focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/30 data-[layout=inline]:items-end data-[layout=stacked]:flex-col">
      <textarea
        ref={inputRef}
        aria-label="Message"
        autoComplete="off"
        name="message"
        rows={1}
        value={value}
        placeholder={placeholder}
        disabled={disabled || busy || switching}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={(event) => { if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); onSubmit(); } }}
        /* The app-wide focus outline is unlayered CSS, so only an important
           utility can drop it here; the ring around the whole box below is what
           shows the focus instead, and a second ring hugging the text would
           read as two controls. */
        className="w-full min-w-0 flex-1 resize-none bg-transparent px-2.5 py-2.5 text-base leading-6 placeholder:text-muted-foreground focus-visible:outline-none! disabled:cursor-not-allowed disabled:opacity-60 md:text-sm"
      />
      <div className="flex shrink-0 items-center gap-2 group-data-[layout=stacked]:justify-between">
        <Select value={agentId} disabled={switching} onValueChange={onAgentChange}>
          <SelectTrigger aria-label="Choose assistant" data-playground-context-trigger="assistant" className="w-auto max-w-40 gap-1 rounded-full sm:max-w-56 border-0 bg-transparent px-3 shadow-none hover:bg-accent">
            <SelectValue placeholder="Choose a ready assistant" />
          </SelectTrigger>
          <SelectContent>{agents.map((agent) => <SelectItem key={agent.id} value={agent.id} disabled={!agent.ready}>{agent.name}{agent.ready ? "" : " — not ready"}</SelectItem>)}</SelectContent>
        </Select>
        {busy
          ? <StopRunButton disabled={switching} onStop={onStop} />
          : <Button type="button" size="icon" aria-label="Send" title="Send" className="rounded-full" disabled={disabled || !value.trim()} onClick={onSubmit}><ArrowUp /></Button>}
      </div>
    </div>
  );
}
