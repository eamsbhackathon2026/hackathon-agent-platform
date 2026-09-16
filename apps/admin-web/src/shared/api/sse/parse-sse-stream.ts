export type SseMessage = {
  event: string;
  id?: string;
  data: string;
};

function parseBlock(block: string): SseMessage | null {
  let event = "message";
  let id: string | undefined;
  const data: string[] = [];

  for (const line of block.split("\n")) {
    if (!line || line.startsWith(":")) continue;
    const separator = line.indexOf(":");
    const field = separator < 0 ? line : line.slice(0, separator);
    let value = separator < 0 ? "" : line.slice(separator + 1);
    if (value.startsWith(" ")) value = value.slice(1);

    if (field === "event") event = value;
    else if (field === "id" && !value.includes("\0")) id = value;
    else if (field === "data") data.push(value);
  }

  return data.length ? { event, ...(id === undefined ? {} : { id }), data: data.join("\n") } : null;
}

export async function* parseSseStream(
  stream: ReadableStream<Uint8Array>,
): AsyncGenerator<SseMessage> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let pendingCarriageReturn = false;

  try {
    while (true) {
      const { done, value } = await reader.read();
      let decoded = decoder.decode(value, { stream: !done });
      if (pendingCarriageReturn) {
        decoded = `\r${decoded}`;
        pendingCarriageReturn = false;
      }
      if (!done && decoded.endsWith("\r")) {
        decoded = decoded.slice(0, -1);
        pendingCarriageReturn = true;
      }
      buffer += decoded.replaceAll("\r\n", "\n").replaceAll("\r", "\n");

      let boundary = buffer.indexOf("\n\n");
      while (boundary >= 0) {
        const message = parseBlock(buffer.slice(0, boundary));
        buffer = buffer.slice(boundary + 2);
        if (message) yield message;
        boundary = buffer.indexOf("\n\n");
      }

      if (done) {
        if (pendingCarriageReturn) buffer += "\n";
        const message = parseBlock(buffer);
        if (message) yield message;
        return;
      }
    }
  } finally {
    reader.releaseLock();
  }
}
