import type { Task } from "@/lib/types";

// taskLabel renders a concise, identifiable task label used by every task filter
// (findings / dashboard / assets). Prefers the user-set Name/Objective; falls back to
// "#id Description(Cut)" so a task is always uniquely recognizable.
export function taskLabel(t: { id: string; name?: string; description?: string }): string {
  const name = (t.name ?? "").trim();
  const desc = (t.description ?? "").trim();
  const body = name || (desc.length > 28 ? desc.slice(0, 28) + "…" : desc) || "Unnamed Task";
  return `#${t.id} ${body}`; // Numbering(Distinction) + Name Priority/Description Cut
}

// taskMap builds an id→Task lookup for resolving labels from a task list.
export function taskMap(tasks: Task[]): Map<string, Task> {
  return new Map(tasks.map((t) => [t.id, t]));
}
