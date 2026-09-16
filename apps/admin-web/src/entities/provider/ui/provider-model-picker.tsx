import { Check, ChevronsUpDown } from "lucide-react";
import { useId, useState } from "react";
import { Button, Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, Input, Label, Popover, PopoverContent, PopoverTrigger } from "@/shared/ui";
import { modelLogo, type ProviderModel } from "../model/provider-catalog";

type Props = {
  models: ProviderModel[];
  value: string;
  onChange: (value: string) => void;
  name?: string;
  label?: string;
  error?: string | undefined;
  display?: "cards" | "combobox";
};

export function ProviderModelPicker({ models, value, onChange, name = "model", label = "AI model", error, display = "cards" }: Props) {
  const inputId = useId();
  if (display === "combobox" && models.length) return <ModelCombobox inputId={inputId} label={label} models={models} name={name} value={value} error={error} onChange={onChange} />;
  return <div className="grid gap-2">
    <Label htmlFor={inputId}>{label}</Label>
    <Input
      aria-invalid={Boolean(error)}
      autoComplete="off"
      id={inputId}
      name={name}
      placeholder="Choose from the list or enter a name"
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
    {models.length ? <div aria-label="Available models" className="grid gap-2" role="listbox">
      {models.map((model) => {
        const selected = value === model.id;
        const logo = modelLogo(model.id);
        return <button
          aria-selected={selected}
          className="flex min-h-12 items-center gap-3 rounded-lg border bg-background px-3 py-2 text-left transition-colors hover:border-primary/50 hover:bg-accent/50 aria-selected:border-primary aria-selected:bg-primary/5"
          key={model.id}
          onClick={() => onChange(model.id)}
          role="option"
          type="button"
        >
          <ModelLogo logo={logo} />
          <span className="min-w-0 flex-1"><span className="block text-sm font-medium">{model.display_name}</span><span className="block truncate text-xs text-muted-foreground">{model.id}</span></span>
          {selected ? <Check aria-hidden="true" className="size-4 text-primary" /> : null}
        </button>;
      })}
    </div> : null}
    {error ? <span className="text-sm text-destructive">{error}</span> : null}
  </div>;
}

function ModelCombobox({ inputId, label, models, name, value, error, onChange }: Props & { inputId: string; label: string; name: string }) {
  const errorId = `${inputId}-error`;
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const selected = models.find((model) => model.id === value);
  const customModel = search.trim();
  const normalizedSearch = customModel.toLowerCase();
  const hasSearchMatch = models.some((model) => model.id.toLowerCase().includes(normalizedSearch) || model.display_name.toLowerCase().includes(normalizedSearch));
  const choose = (modelId: string) => {
    onChange(modelId);
    setOpen(false);
    setSearch("");
  };

  return <div className="grid gap-2">
    <Label htmlFor={inputId}>{label}</Label>
    <input name={name} type="hidden" value={value} />
    <Popover open={open} onOpenChange={(next) => { setOpen(next); if (!next) setSearch(""); }}>
      <PopoverTrigger asChild>
        <Button
          aria-controls={`${inputId}-options`}
          aria-describedby={error ? errorId : undefined}
          aria-expanded={open}
          aria-invalid={Boolean(error)}
          className="h-auto min-h-11 w-full justify-between gap-3 px-3 py-2 font-normal"
          id={inputId}
          role="combobox"
          type="button"
          variant="outline"
        >
          <span className="flex min-w-0 items-center gap-2.5">
            {value ? <ModelLogo logo={modelLogo(value)} /> : null}
            <span className="min-w-0 text-left">
              <span className="block truncate text-sm font-medium">{(selected?.display_name ?? value) || "Choose a model"}</span>
              {value ? <span className="block truncate text-xs text-muted-foreground">{value}</span> : null}
            </span>
          </span>
          <ChevronsUpDown aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-[var(--radix-popover-trigger-width)] min-w-72 max-w-[calc(100vw-2rem)] p-0">
        <Command>
          <CommandInput autoFocus placeholder="Search models..." value={search} onValueChange={setSearch} />
          <CommandList id={`${inputId}-options`}>
            <CommandEmpty>No matching model.</CommandEmpty>
            <CommandGroup heading="Available models">
              {models.map((model) => <CommandItem key={model.id} value={`${model.display_name} ${model.id}`} onSelect={() => choose(model.id)}>
                <ModelLogo logo={modelLogo(model.id)} />
                <span className="min-w-0 flex-1"><span className="block font-medium">{model.display_name}</span><span className="block truncate text-xs text-muted-foreground">{model.id}</span></span>
                <Check aria-hidden="true" className={value === model.id ? "ml-auto size-4 opacity-100" : "ml-auto size-4 opacity-0"} />
              </CommandItem>)}
              {customModel && !hasSearchMatch ? <CommandItem value={`custom ${customModel}`} onSelect={() => choose(customModel)}>
                <span className="flex size-7 items-center justify-center rounded-md bg-muted text-xs font-semibold">AI</span>
                <span className="min-w-0 flex-1">Use custom model <span className="block truncate text-xs text-muted-foreground">{customModel}</span></span>
              </CommandItem> : null}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
    {error ? <span className="text-sm text-destructive" id={errorId}>{error}</span> : null}
  </div>;
}

function ModelLogo({ logo }: { logo: ReturnType<typeof modelLogo> }) {
  if (!logo) return <span aria-hidden="true" className="flex size-8 items-center justify-center rounded-md bg-muted text-xs font-semibold">AI</span>;
  if (logo.monochrome) return <span aria-label={logo.label} className="size-7 shrink-0 bg-foreground" role="img" style={{ mask: `url(${logo.src}) center / contain no-repeat`, WebkitMask: `url(${logo.src}) center / contain no-repeat` }} />;
  return <img alt={`${logo.label} logo`} className="size-7 shrink-0 object-contain" src={logo.src} />;
}
