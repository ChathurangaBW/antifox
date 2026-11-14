"use client";

import { useTranslation } from "react-i18next";

import { StatusBadge } from "@/components/status-badge";
import type { TaskObservation } from "@/hooks/use-task-observation";

export function TaskObservationStatus({ observation }: { observation: TaskObservation }) {
  const { t } = useTranslation();
  const { task, stats, freshness } = observation;
  if (freshness === "loading" || freshness === "unavailable" || !task || !stats) {
    return <span className="text-xs text-muted-foreground">{t("taskDetail.observation.unavailable")}</span>;
  }
  if (["done", "failed", "timeout"].includes(task.status)) {
    return <StatusBadge domain="task" value={task.status} dot />;
  }
  const mode = task.status === "paused" || task.paused || stats.active_task?.paused
    ? "paused" : stats.engine_mode;
  if (mode === "stalled") {
    return <span className="rounded-md border px-2 py-0.5 text-xs text-amber-600">{t("taskDetail.observation.inactive")}</span>;
  }
  return <StatusBadge domain="engine" value={mode} dot />;
}

export function TaskObservationFreshness({ observation }: { observation: TaskObservation }) {
  const { t, i18n } = useTranslation();
  const { observedAt, freshness } = observation;
  return (
    <p role="status" className="text-xs text-muted-foreground">
      {freshness === "loading" ? t("common.loading")
        : freshness === "unavailable" ? t("taskDetail.observation.unavailable")
        : t(`taskDetail.observation.${freshness}`, {
          time: observedAt ? new Date(observedAt).toLocaleTimeString(i18n.resolvedLanguage) : "—",
        })}
    </p>
  );
}
