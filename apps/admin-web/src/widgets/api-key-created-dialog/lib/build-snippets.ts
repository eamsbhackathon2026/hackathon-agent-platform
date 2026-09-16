export type SnippetInput = { baseUrl: string; apiKey: string; agentId: string };

export function buildSnippets({ baseUrl, apiKey, agentId }: SnippetInput) {
  const endpoint = `${baseUrl.replace(/\/$/, "")}/v1/agents/${agentId}/runs`;
  const auth = `-H 'X-API-Key: ${apiKey}'`;
  const syncInput = `-d '{"input":{"message":"Hello"},"mode":"sync"}'`;
  const streamInput = `-d '{"input":{"message":"Hello"}}'`;
  return {
    sync: `curl '${endpoint}' ${auth} -H 'Content-Type: application/json' ${syncInput}`,
    stream: `curl -N '${endpoint}/stream' ${auth} -H 'Content-Type: application/json' ${streamInput}`,
    async: `curl '${endpoint}' ${auth} -H 'Content-Type: application/json' -H 'Idempotency-Key: request-001' -d '{"input":{"message":"Hello"},"mode":"async","webhook_url":"https://example.com/hooks/agent"}'`,
    nodeVerify: `const { Webhook } = require("standardwebhooks");
const webhook = new Webhook(process.env.WEBHOOK_SECRET);

async function acceptWebhook(rawBody, headers, claimWebhook) {
  const id = headers["webhook-id"];
  const timestamp = Number(headers["webhook-timestamp"]);
  if (!id || !Number.isFinite(timestamp) || Math.abs(Date.now() / 1000 - timestamp) > 300) {
    throw new Error("Webhook timestamp is outside the 5-minute window");
  }
  const payload = webhook.verify(rawBody, headers);
  // claimWebhook must atomically insert webhook-id into durable storage with a unique constraint.
  if (!(await claimWebhook(id))) return { duplicate: true };
  // Perform side effects only after this request owns the durable claim.
  return { duplicate: false, payload };
}`,
    goVerify: `// claimWebhook must atomically insert id into durable storage with a unique constraint.
func verify(body []byte, id, timestamp, signature string, claimWebhook func(string) bool) (valid, duplicate bool) {
  if id == "" || claimWebhook == nil { return false, false }
  unixSeconds, err := strconv.ParseInt(timestamp, 10, 64)
  if err != nil || math.Abs(time.Since(time.Unix(unixSeconds, 0)).Seconds()) > 300 { return false, false }
  encoded := strings.TrimPrefix(os.Getenv("WEBHOOK_SECRET"), "whsec_")
  secret, err := base64.StdEncoding.DecodeString(encoded)
  if err != nil { return false, false }
  mac := hmac.New(sha256.New, secret)
  mac.Write([]byte(id + "." + timestamp + "."))
  mac.Write(body)
  expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
  for _, candidate := range strings.Fields(signature) {
    parts := strings.SplitN(candidate, ",", 2)
    if len(parts) == 2 && parts[0] == "v1" && hmac.Equal([]byte(parts[1]), []byte(expected)) {
      // Perform side effects only when this request owns the durable claim.
      if !claimWebhook(id) { return true, true }
      return true, false
    }
  }
  return false, false
}`,
  };
}
