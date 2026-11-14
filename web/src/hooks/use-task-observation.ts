"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { api } from "@/lib/api";
import type { Stats, Task } from "@/lib/types";

export interface TaskObservation {
  taskId: string;
  task: Task | null;
  stats: Stats | null;
  observedAt: number | null;
  freshness: "loading" | "fresh" | "stale" | "unavailable";
}

const emptyObservation = (taskId: string): TaskObservation => ({
  taskId, task: null, stats: null, observedAt: null, freshness: "loading",
});

// One read-only observation drives both the detail header and Overview.
export function useTaskObservation(taskId: string) {
  const [observation, setObservation] = useState(() => emptyObservation(taskId));
  const refreshRef = useRef<() => void>(() => {});
  const refresh = useCallback(() => refreshRef.current(), []);

  useEffect(() => {
    let alive = true;
    let generation = 0;
    let controller: AbortController | undefined;
    let pollTimer: ReturnType<typeof setTimeout> | undefined;
    let staleTimer: ReturnType<typeof setTimeout> | undefined;

    const load = async () => {
      const request = ++generation;
      clearTimeout(pollTimer);
      controller?.abort();
      const current = new AbortController();
      controller = current;
      const timeout = setTimeout(() => current.abort(), 10000);
      try {
        const [tasks, stats] = await Promise.all([
          api.tasks(current.signal), api.stats(taskId, current.signal),
        ]);
        if (!alive || request !== generation || current.signal.aborted) return;
        const task = tasks.tasks.find((row) => row.id === taskId) ?? null;
        if (task && stats.active_task?.id !== taskId) {
          throw new Error("Task observation does not match the selected task");
        }
        setObservation({ taskId, task, stats, observedAt: Date.now(), freshness: "fresh" });
        clearTimeout(staleTimer);
        staleTimer = setTimeout(() => {
          if (alive) setObservation((previous) => ({ ...previous, freshness: "stale" }));
        }, 10000);
      } catch {
        if (!alive || request !== generation) return;
        current.abort();
        setObservation((previous) => {
          const retained = previous.taskId === taskId ? previous : emptyObservation(taskId);
          return { ...retained, freshness: retained.observedAt ? "stale" : "unavailable" };
        });
      } finally {
        clearTimeout(timeout);
        if (alive && request === generation) pollTimer = setTimeout(load, 3000);
      }
    };

    refreshRef.current = () => { void load(); };
    void load();
    return () => {
      alive = false;
      generation++;
      controller?.abort();
      clearTimeout(pollTimer);
      clearTimeout(staleTimer);
      refreshRef.current = () => {};
    };
  }, [taskId]);

  // Never display a previous task while the new task's first request is pending.
  return { ...(observation.taskId === taskId ? observation : emptyObservation(taskId)), refresh };
}
