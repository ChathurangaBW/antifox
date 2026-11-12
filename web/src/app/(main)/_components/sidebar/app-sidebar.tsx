"use client";

import Link from "next/link";

import { useShallow } from "zustand/react/shallow";

import { useMemo } from "react";

import { useTranslation } from "react-i18next";

import { AntifoxLogo } from "@/components/antifox-logo";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { APP_CONFIG } from "@/config/app-config";
import { useCurrentUser } from "@/hooks/use-current-user";
import { usePermissions } from "@/hooks/use-permissions";
import { sidebarItems } from "@/navigation/sidebar/sidebar-items";
import { usePreferencesStore } from "@/stores/preferences/preferences-provider";

import { NavMain } from "./nav-main";
import { NavUser } from "./nav-user";
import { SidebarSupportCard } from "./sidebar-support-card";


// nav Item → The required read permissions (no mapping items are always shown). Use to hide unauthorized pages by character.
const NAV_PERM: Record<string, string> = {
  dashboard: "dashboard:read",
  chat: "task:read",
  tasks: "task:read",
  findings: "finding:read",
  traffic: "traffic:read",
  assets: "asset:read",
  sync: "asset:read",
  llm: "llm:read",
  agents: "agent:read",
  mcp: "mcp:read",
  skills: "skill:read",
  tools: "tool:read",
  intercept: "intercept:read",
  approvals: "intercept:read",
  rbac: "rbac:read",
  reports: "report:read",
  knowledge: "knowledge:read",
  logs: "log:read",
  settings: "config:read",
};

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const currentUser = useCurrentUser();
  const { ready, has } = usePermissions();
  const { t, i18n } = useTranslation();
  const { sidebarVariant, sidebarCollapsible, isSynced } = usePreferencesStore(
    useShallow((s) => ({
      sidebarVariant: s.sidebarVariant,
      sidebarCollapsible: s.sidebarCollapsible,
      isSynced: s.isSynced,
    })),
  );

  const variant = isSynced ? sidebarVariant : props.variant;
  const collapsible = isSynced ? sidebarCollapsible : props.collapsible;

  // Filter with Permission nav + Translator id Cha. nav.* / navGroup.*,In Chinese, in the original language.
  // Shows all rights before they are ready (avoiding blinking) and hides no rights after they are ready.i18n.language Dependence, toggle instant recalculation.
  const navItems = useMemo(() => {
    const tr = <T extends { id: string; title: string }>(it: T): T => ({
      ...it,
      title: t(`nav.${it.id}`, { defaultValue: it.title }),
    });
    return sidebarItems
      .map((g) => ({
        ...g,
        label: g.label ? t(`navGroup.${g.id}`, { defaultValue: g.label }) : g.label,
        items: g.items
          .filter((it) => !ready || !NAV_PERM[it.id] || has(NAV_PERM[it.id]))
          .map((it) => ("subItems" in it && it.subItems ? { ...tr(it), subItems: it.subItems.map(tr) } : tr(it))),
      }))
      .filter((g) => g.items.length > 0);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, has, t, i18n.language]);

  return (
    <Sidebar {...props} variant={variant} collapsible={collapsible}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton asChild>
              <Link prefetch={false} href="/function/tasks">
                <AntifoxLogo className="size-8 shrink-0" />
                <span className="font-semibold text-base">{APP_CONFIG.name}</span>
                <span className="rounded-full border border-primary/20 bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium leading-none text-primary">
                  {APP_CONFIG.version}
                </span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={navItems} />
        {/* <NavDocuments items={data.documents} /> */}
        {/* <NavSecondary items={data.navSecondary} className="mt-auto" /> */}
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={currentUser} />
      </SidebarFooter>
    </Sidebar>
  );
}
