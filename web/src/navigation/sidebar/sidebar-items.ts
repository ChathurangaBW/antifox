import {
  Activity,
  Bot,
  Brain,
  Bug,
  ClipboardList,
  FileText,
  FolderSync,
  LayoutDashboard,
  type LucideIcon,
  MessageSquare,
  Network,
  Plug,
  ScrollText,
  Settings2,
  ShieldAlert,
  Sparkles,
  Target,
  UsersRound,
  Wrench,
} from "lucide-react";

export type NavBadge = "new" | "soon";

export interface NavSubItem {
  id: string;
  title: string;
  url: string;
  icon?: LucideIcon;
  badge?: NavBadge;
  disabled?: boolean;
  newTab?: boolean;
}

interface NavItemBase {
  id: string;
  title: string;
  icon?: LucideIcon;
  badge?: NavBadge;
  disabled?: boolean;
  newTab?: boolean;
}

export interface NavMainLinkItem extends NavItemBase {
  url: string;
  subItems?: never;
}

export interface NavMainParentItem extends NavItemBase {
  subItems: NavSubItem[];
}

export type NavMainItem = NavMainLinkItem | NavMainParentItem;

export interface NavGroup {
  id: number;
  label?: string;
  items: NavMainItem[];
}

export const sidebarItems: NavGroup[] = [
  {
    id: 1,
    label: "Functions",
    items: [
      { id: "dashboard", title: "dashboard", url: "/dashboard", icon: LayoutDashboard },
      { id: "chat", title: "Dialogue", url: "/chat", icon: MessageSquare },
      { id: "tasks", title: "Tasks", url: "/function/tasks", icon: Target },
      { id: "findings", title: "Found", url: "/function/findings", icon: Bug },
      { id: "traffic", title: "Traffic", url: "/function/traffic", icon: Activity },
      { id: "assets", title: "Assets", url: "/function/assets", icon: Network },
      { id: "sync", title: "Asset Sync", url: "/function/sync", icon: FolderSync },
    ],
  },
  {
    id: 2,
    label: "System",
    items: [
      { id: "llm", title: "LLM", url: "/system/llm", icon: Brain },
      { id: "agents", title: "Agent", url: "/system/agents", icon: Bot },
      { id: "mcp", title: "MCP", url: "/system/mcp", icon: Plug },
      { id: "skills", title: "Skill", url: "/system/skills", icon: Sparkles },
      { id: "tools", title: "Tools", url: "/system/tools", icon: Wrench },
      { id: "intercept", title: "Interception rules", url: "/system/intercept", icon: ShieldAlert },
      { id: "approvals", title: "Approval records", url: "/system/intercept/approvals", icon: ClipboardList },
      { id: "rbac", title: "User Management", url: "/system/rbac", icon: UsersRound },
      { id: "reports", title: "Reporting management", url: "/system/reports", icon: FileText },
      { id: "knowledge", title: "Knowledge base", url: "/system/knowledge", icon: Brain },
      { id: "logs", title: "Log", url: "/system/logs", icon: ScrollText },
      { id: "settings", title: "System Configuration", url: "/system/settings", icon: Settings2 },
    ],
  },
];
