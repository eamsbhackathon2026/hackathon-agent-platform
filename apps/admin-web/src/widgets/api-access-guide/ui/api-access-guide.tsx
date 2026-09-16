import { Activity, ChevronDown, KeyRound, Radio, Send, ShieldCheck } from "lucide-react";
import { useState, type ReactNode } from "react";
import { Link } from "react-router";

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/shared/ui";

type Props = { onCreateKey: () => void };

const steps = [
  {
    title: "Create a scoped key",
    description:
      "Grant Send requests to assistants. Add View status and results when your application polls or cancels runs.",
  },
  {
    title: "Save both secrets",
    description:
      "Copy the access key and delivery signing secret when they appear. They cannot be viewed again.",
  },
  {
    title: "Call a ready assistant",
    description:
      "Send the access key from your server and use the assistant ID in the run endpoint.",
  },
  {
    title: "Handle and monitor results",
    description:
      "Wait for a response, stream updates, or receive a background delivery, then inspect Activity when needed.",
  },
] as const;

const requestExample = `curl "$AGENT_PLATFORM_URL/v1/agents/$AGENT_ID/runs" \\
  -H "X-API-Key: $AGENT_PLATFORM_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: request-001" \\
  -d '{"input":{"message":"Hello"},"mode":"sync"}'`;

export function ApiAccessGuide({ onCreateKey }: Props) {
  const [open, setOpen] = useState(false);

  return (
    <section aria-labelledby="api-access-guide-title">
      <Collapsible open={open} onOpenChange={setOpen}>
        <Card className="overflow-hidden">
          <CardHeader className="gap-4 space-y-0 bg-muted/35 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0 space-y-2">
              <Badge variant="outline">Integration guide</Badge>
              <h2 id="api-access-guide-title" className="text-xl font-semibold leading-tight tracking-[-0.015em]">
                Connect your application
              </h2>
              <CardDescription className="max-w-3xl">
                Open this guide when you need API setup steps, request examples, or delivery details.
              </CardDescription>
            </div>
            <CollapsibleTrigger asChild>
              <Button type="button" variant="outline" className="w-full shrink-0 justify-between sm:w-auto">
                {open ? "Hide integration guide" : "View integration guide"}
                <ChevronDown
                  aria-hidden="true"
                  className={`transition-transform duration-200 ${open ? "rotate-180" : ""}`}
                />
              </Button>
            </CollapsibleTrigger>
          </CardHeader>

          <CollapsibleContent>
            <CardContent className="space-y-6 border-t pt-5 sm:pt-6">
              <ol className="grid gap-px overflow-hidden rounded-2xl border bg-border md:grid-cols-2 xl:grid-cols-4">
                {steps.map((step, index) => (
                  <li key={step.title} className="bg-card p-4">
                    <span className="text-xs font-semibold tracking-[0.12em] text-primary">
                      STEP {index + 1}
                    </span>
                    <h3 className="mt-2 font-semibold">{step.title}</h3>
                    <p className="mt-1.5 text-sm leading-6 text-muted-foreground">{step.description}</p>
                  </li>
                ))}
              </ol>

              <div className="grid gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(18rem,0.65fr)]">
                <section
                  className="min-w-0 rounded-2xl bg-sidebar p-5 text-sidebar-foreground"
                  aria-labelledby="api-request-example-title"
                >
                  <div className="flex items-center gap-2">
                    <Send aria-hidden="true" className="size-4 text-sidebar-accent" />
                    <h3 id="api-request-example-title" className="font-semibold">
                      Send your first request
                    </h3>
                  </div>
                  <p className="mt-2 text-sm leading-6 text-sidebar-muted">
                    Set the three environment variables on your server, then run this immediate request.
                  </p>
                  <pre className="mt-4 max-w-full overflow-x-auto rounded-xl border border-sidebar-foreground/10 bg-sidebar-foreground/5 p-4 text-xs leading-6 text-sidebar-foreground">
                    <code>{requestExample}</code>
                  </pre>
                  <p className="mt-3 text-xs leading-5 text-sidebar-muted">
                    Reuse the same Idempotency-Key only when retrying the same non-streaming request.
                  </p>
                </section>

                <section className="rounded-2xl border bg-muted/25 p-5" aria-labelledby="delivery-mode-title">
                  <div className="flex items-center gap-2">
                    <Radio aria-hidden="true" className="size-4 text-primary" />
                    <h3 id="delivery-mode-title" className="font-semibold">
                      Choose how results arrive
                    </h3>
                  </div>
                  <dl className="mt-4 space-y-4">
                    <div>
                      <dt className="font-medium">Immediate</dt>
                      <dd className="mt-1 text-sm leading-6 text-muted-foreground">
                        Use <InlineCode>mode: &quot;sync&quot;</InlineCode> and wait for the completed run.
                      </dd>
                    </div>
                    <div>
                      <dt className="font-medium">Live stream</dt>
                      <dd className="mt-1 text-sm leading-6 text-muted-foreground">
                        Call <InlineCode>/runs/stream</InlineCode> and read Server-Sent Events until the final event.
                      </dd>
                    </div>
                    <div>
                      <dt className="font-medium">Background</dt>
                      <dd className="mt-1 text-sm leading-6 text-muted-foreground">
                        Use <InlineCode>mode: &quot;async&quot;</InlineCode>. Polling the Location header requires View
                        status and results; a delivery endpoint does not.
                      </dd>
                    </div>
                  </dl>
                </section>
              </div>

              <div className="flex gap-3 rounded-2xl border bg-muted/35 p-4">
                <ShieldCheck aria-hidden="true" className="mt-0.5 size-5 shrink-0 text-primary" />
                <div>
                  <h3 className="font-semibold">Protect every key</h3>
                  <p className="mt-1 text-sm leading-6 text-muted-foreground">
                    Never expose an access key in browser or mobile code. For background deliveries, verify the
                    signature against the raw body before parsing, reject timestamps more than five minutes apart,
                    and atomically deduplicate each webhook ID before causing side effects.
                  </p>
                </div>
              </div>

              <div className="flex flex-col gap-2 sm:flex-row">
                <Button type="button" variant="outline" onClick={onCreateKey}>
                  <KeyRound aria-hidden="true" />
                  Create access key
                </Button>
                <Button asChild variant="ghost">
                  <Link to="/activity">
                    <Activity aria-hidden="true" />
                    View activity
                  </Link>
                </Button>
              </div>
            </CardContent>
          </CollapsibleContent>
        </Card>
      </Collapsible>
    </section>
  );
}

function InlineCode({ children }: { children: ReactNode }) {
  return <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs text-foreground">{children}</code>;
}
