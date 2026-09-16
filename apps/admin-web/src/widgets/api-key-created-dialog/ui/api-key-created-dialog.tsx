import { useId, useMemo, useState } from "react";
import { Link } from "react-router";

import type { ApiKeyCreated } from "@/entities/api-key";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  Input,
  Label,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/shared/ui";

import { buildSnippets } from "../lib/build-snippets";

type Props = { created: ApiKeyCreated | null; agentId: string; onClose: () => void };

export function ApiKeyCreatedDialog({ created, agentId, onClose }: Props) {
  const fieldId = useId();
  const [copied, setCopied] = useState("");
  const snippets = useMemo(
    () =>
      created
        ? buildSnippets({ baseUrl: window.location.origin, apiKey: created.key, agentId })
        : null,
    [created, agentId],
  );
  const copy = async (name: string, value: string) => {
    await navigator.clipboard.writeText(value);
    setCopied(name);
  };

  return (
    <Dialog
      open={Boolean(created)}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="max-h-[calc(100dvh-2rem)] max-w-3xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Save your access key now</DialogTitle>
          <DialogDescription>
            The key and signing secret are shown only once. You cannot view them again after closing this
            window.
          </DialogDescription>
        </DialogHeader>
        {created && snippets ? (
          <div className="min-w-0 space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor={`${fieldId}-access-key`}>Access key</Label>
              <div className="flex min-w-0 gap-2">
                <Input
                  id={`${fieldId}-access-key`}
                  className="min-w-0 flex-1 font-mono"
                  readOnly
                  value={created.key}
                />
                <Button
                  className="shrink-0"
                  type="button"
                  variant="outline"
                  onClick={() => copy("key", created.key)}
                >
                  {copied === "key" ? "Copied" : "Copy"}
                </Button>
              </div>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor={`${fieldId}-signing-secret`}>Delivery signing secret</Label>
              <div className="flex min-w-0 gap-2">
                <Input
                  id={`${fieldId}-signing-secret`}
                  className="min-w-0 flex-1 font-mono"
                  readOnly
                  value={created.webhook_secret}
                />
                <Button
                  className="shrink-0"
                  type="button"
                  variant="outline"
                  onClick={() => copy("secret", created.webhook_secret)}
                >
                  {copied === "secret" ? "Copied" : "Copy"}
                </Button>
              </div>
            </div>
            <Tabs className="min-w-0" defaultValue="sync">
              <TabsList className="w-full max-w-full justify-start overflow-x-auto">
                <TabsTrigger className="shrink-0" value="sync">
                  Immediate
                </TabsTrigger>
                <TabsTrigger className="shrink-0" value="stream">
                  Live stream
                </TabsTrigger>
                <TabsTrigger className="shrink-0" value="async">
                  Background
                </TabsTrigger>
                <TabsTrigger className="shrink-0" value="verify">
                  Verify signature
                </TabsTrigger>
              </TabsList>
              <TabsContent className="min-w-0" value="sync">
                <Code value={snippets.sync} />
              </TabsContent>
              <TabsContent className="min-w-0" value="stream">
                <Code value={snippets.stream} />
              </TabsContent>
              <TabsContent className="min-w-0" value="async">
                <Code value={snippets.async} />
              </TabsContent>
              <TabsContent className="min-w-0" value="verify">
                <Code value={`${snippets.nodeVerify}\n\n${snippets.goVerify}`} />
              </TabsContent>
            </Tabs>
            <Button asChild variant="link">
              <Link to="/activity?source=api">View API activity</Link>
            </Button>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function Code({ value }: { value: string }) {
  return (
    <pre className="max-h-56 min-w-0 w-full max-w-full overflow-x-auto overflow-y-auto rounded-lg bg-slate-950 p-4 text-xs text-slate-50">
      <code>{value}</code>
    </pre>
  );
}
