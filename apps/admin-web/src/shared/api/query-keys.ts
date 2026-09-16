export const queryKeys = {
  me: ["me"] as const,
  agents: ["agents"] as const,
  agent: (id: string) => ["agents", id] as const,
  providers: ["providers"] as const,
  provider: (id: string) => ["providers", id] as const,
  skills: ["skills"] as const,
  skill: (id: string) => ["skills", id] as const,
  agentSkills: (id: string) => ["agent-skills", id] as const,
};
