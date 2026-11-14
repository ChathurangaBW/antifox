"use client";

import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  ActivityIcon,
  AlertTriangleIcon,
  BugIcon,
  ClockIcon,
  ShieldCheckIcon,
  TargetIcon,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { StatusBadge } from "@/components/status-badge";
import { TaskObservationFreshness, TaskObservationStatus } from "@/components/task-observation-status";
import type { TaskObservation } from "@/hooks/use-task-observation";
import { api } from "@/lib/api";
import type { TaskNode, Finding } from "@/lib/types";

function StatCard({
  label,
  value,
  sub,
  icon: Icon,
}: {
  label: string;
  value: React.ReactNode;
  sub?: string;
  icon: React.ElementType;
}) {
  return (
    <Card className="gap-1.5">
      <CardHeader className="pb-0">
        <CardDescription className="flex items-center gap-1.5">
          <Icon className="size-3.5" /> {label}
        </CardDescription>
        <CardTitle className="text-2xl tabular-nums">{value}</CardTitle>
      </CardHeader>
      {sub && (
        <CardContent className="text-xs text-muted-foreground">{sub}</CardContent>
      )}
    </Card>
  );
}

export function OverviewTab({ taskId, observation }: { taskId: string; observation: TaskObservation }) {
  const { t, i18n } = useTranslation();
  const { task, stats } = observation;
  const [intents, setIntents] = React.useState<TaskNode[]>([]);
  const [findings, setFindings] = React.useState<Finding[]>([]);
  const [detailsStale, setDetailsStale] = React.useState(false);

  React.useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const load = async () => {
      try {
        const [intentsResp, findingsResp] =
          await Promise.all([
            api.intents(taskId),
            api.findings(taskId),
          ]);
        if (cancelled) return;
        setIntents(intentsResp);
        setFindings(findingsResp);
        setDetailsStale(false);
      } catch {
        if (!cancelled) setDetailsStale(true);
      } finally {
        if (!cancelled) timer = setTimeout(load, 3000);
      }
    };

    load();
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [taskId]);

  const running = intents.filter((i) => i.state === "running");
  const open = intents.filter((i) => i.state === "open");
  const blocked = intents.filter((i) => i.state === "blocked");
  const taskFindings = findings.filter((f) => f.task_id === taskId);
  const goalsPct = task?.goals_total
    ? Math.round(((task.goals_met ?? 0) / task.goals_total) * 100)
    : 0;

  return (
    <div className="flex flex-col gap-4">
      {/* Heartbeat */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <ActivityIcon className="size-4 text-blue-500" /> {t("taskDetail.overview.heartbeat")}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          <div>
            <div className="text-xs text-muted-foreground">{t("taskDetail.overview.engineState")}</div>
            <div className="mt-1"><TaskObservationStatus observation={observation} /></div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">{t("taskDetail.overview.runningWorker")}</div>
            <div className="mt-1 text-lg font-semibold tabular-nums">
              {stats?.active_task?.in_flight ?? "—"}
            </div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">{t("taskDetail.overview.recentActivity")}</div>
            <div className="mt-1 inline-flex items-center gap-1 text-sm">
              <ClockIcon className="size-3.5" />
              {task?.last_activity_unix && task.last_activity_unix > 0
                ? new Date(task.last_activity_unix * 1000).toLocaleString(i18n.resolvedLanguage)
                : "—"}
            </div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">
              {t("taskDetail.overview.goals", { met: task?.goals_met ?? 0, total: task?.goals_total ?? 0 })}
            </div>
            <Progress value={goalsPct} className="mt-2" />
          </div>
          {task?.completed_unix && task.completed_unix > 0 ? (
            <div>
              <div className="text-xs text-muted-foreground">{t("taskDetail.overview.completedAt")}</div>
              <div className="mt-1 inline-flex items-center gap-1 text-sm">
                <ClockIcon className="size-3.5" />
                {new Date(task.completed_unix * 1000).toLocaleString(i18n.resolvedLanguage)}
              </div>
            </div>
          ) : null}
          <div className="col-span-full"><TaskObservationFreshness observation={observation} /></div>
        </CardContent>
      </Card>

      {/* Work set */}
      {detailsStale && <p role="status" className="text-sm text-muted-foreground">{t("taskDetail.observation.detailsStale")}</p>}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <TargetIcon className="size-4" /> {t("taskDetail.overview.runningIntents")}
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            {running.slice(0, 6).map((i) => (
              <div key={i.id} className="flex items-center gap-2 text-sm">
                <StatusBadge domain="intent" value={i.state} />
                <span className="min-w-0 flex-1 truncate">{i.payload}</span>
              </div>
            ))}
            {running.length === 0 && (
              <p className="text-sm text-muted-foreground">{t("taskDetail.overview.noRunningIntents")}</p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <AlertTriangleIcon className="size-4 text-amber-500" /> {t("taskDetail.overview.needsAttention")}
            </CardTitle>
          </CardHeader>
          <CardContent className="grid grid-cols-2 gap-3 text-sm">
            <div>
              <div className="text-2xl font-semibold tabular-nums text-red-600">
                {taskFindings.length}
              </div>
              <div className="text-xs text-muted-foreground">{t("taskDetail.overview.confirmedVulns")}</div>
            </div>
            <div>
              <div className="text-2xl font-semibold tabular-nums text-blue-600">
                {running.length}
              </div>
              <div className="text-xs text-muted-foreground">{t("taskDetail.overview.executing")}</div>
            </div>
            <div>
              <div className="text-2xl font-semibold tabular-nums">
                {open.length}
              </div>
              <div className="text-xs text-muted-foreground">{t("taskDetail.overview.frontierPending")}</div>
            </div>
            <div>
              <div className="text-2xl font-semibold tabular-nums text-red-600">
                {blocked.length}
              </div>
              <div className="text-xs text-muted-foreground">{t("taskDetail.overview.blockedIntents")}</div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <BugIcon className="size-4 text-red-500" /> {t("taskDetail.overview.recentFindings")}
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            {taskFindings.slice(0, 6).map((f) => (
              <div key={f.id} className="flex items-center gap-2 text-sm">
                <StatusBadge domain="severity" value={f.severity} dot />
                <span className="min-w-0 flex-1 truncate">{f.summary}</span>
              </div>
            ))}
            {taskFindings.length === 0 && (
              <p className="text-sm text-muted-foreground">{t("taskDetail.overview.noFindings")}</p>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-3">
        <StatCard
          label={t("taskDetail.overview.stat.pendingIntents")}
          value={open.length}
          icon={ShieldCheckIcon}
          sub={t("taskDetail.overview.stat.frontierOpen")}
        />
        <StatCard
          label={t("taskDetail.overview.stat.confirmedFindings")}
          value={taskFindings.length}
          icon={BugIcon}
          sub={t("taskDetail.overview.stat.thisTask")}
        />
        <StatCard
          label={t("taskDetail.overview.stat.totalIntents")}
          value={intents.length}
          icon={AlertTriangleIcon}
          sub={t("taskDetail.overview.stat.allIntents")}
        />
      </div>
    </div>
  );
}
