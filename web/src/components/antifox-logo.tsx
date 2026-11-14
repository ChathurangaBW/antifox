import { cn } from "@/lib/utils";

// Guardian shield artwork uses square bounds and retains its proportions at every size.
export function AntifoxLogo({ className }: { className?: string }) {
  return (
    // biome-ignore lint/performance/noImgElement: Static logo is served directly by the embedded Go UI.
    <img src="/logo.png" alt="AntiFox" width={512} height={512} className={cn("object-contain", className)} />
  );
}
