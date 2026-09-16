import { describe, expect, it } from "vitest";

import { parseSseStream } from "./parse-sse-stream";

function stream(...chunks: Uint8Array[]) {
  return new ReadableStream<Uint8Array>({
    start(controller) {
      chunks.forEach((chunk) => controller.enqueue(chunk));
      controller.close();
    },
  });
}

async function collect(input: ReadableStream<Uint8Array>) {
  const messages = [];
  for await (const message of parseSseStream(input)) messages.push(message);
  return messages;
}

describe("parseSseStream", () => {
  it("parses fields split across chunks and joins multiple data lines", async () => {
    const encoder = new TextEncoder();
    const result = await collect(stream(
      encoder.encode("event: message.delta\nid: 1\nda"),
      encoder.encode("ta: hello\ndata: world\n\n"),
    ));
    expect(result).toEqual([{ event: "message.delta", id: "1", data: "hello\nworld" }]);
  });

  it("preserves a multi-byte character split between byte chunks", async () => {
    const bytes = new TextEncoder().encode("data: tiếng Việt\n\n");
    const split = bytes.indexOf(0xe1) + 1;
    expect(await collect(stream(bytes.slice(0, split), bytes.slice(split)))).toEqual([
      { event: "message", data: "tiếng Việt" },
    ]);
  });

  it("accepts CRLF and ignores heartbeat comments", async () => {
    const encoder = new TextEncoder();
    expect(await collect(stream(encoder.encode(": ping\r\n\r\nevent: done\r\ndata: {}\r\n\r\n")))).toEqual([
      { event: "done", data: "{}" },
    ]);
  });

  it("does not create a false boundary when CRLF is split between chunks", async () => {
    const encoder = new TextEncoder();
    const result = await collect(stream(
      encoder.encode("event: update\r"),
      encoder.encode("\ndata: first\r"),
      encoder.encode("\n\r"),
      encoder.encode("\ndata: second\r\n\r\n"),
    ));
    expect(result).toEqual([
      { event: "update", data: "first" },
      { event: "message", data: "second" },
    ]);
  });
});
