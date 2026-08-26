/* One list, shared by the hero diagram and the MCP section, so the two can't
 * drift apart. Every mark here is monochrome and inherits currentColor, which
 * is what lets them sit as a quiet row in either theme. */

export interface Agent {
  label: string;
  icon: string;
}

export const agents: Agent[] = [
  { label: "Claude", icon: "simple-icons:anthropic" },
  { label: "ChatGPT", icon: "simple-icons:openai" },
  { label: "Gemini", icon: "simple-icons:googlegemini" },
  { label: "Cursor", icon: "simple-icons:cursor" },
  { label: "Copilot", icon: "simple-icons:githubcopilot" },
  // simple-icons has no Grok mark; logos: carries it and is currentColor too.
  { label: "Grok", icon: "logos:grok-icon" },
];
