import { cn } from "@/lib/utils";

// Brand colours sampled from the guardian shield logo (web/public/logo.png).
export const brand = {
  charcoal: "#1c1d21",
  ink: "#0f1013",
  cream: "#f3e6d3",
  orange: "#e0601a",
  ember: "#c2410c",
};

// Shield outline in a 100 x 110 box, the same silhouette as the logo.
const SHIELD = "M50 4 L92 18 Q92 70 50 106 Q8 70 8 18 Z";

// BrandBackdrop fills its (relative) parent with the branded background used by
// the login and setup screens: charcoal field, an ember glow behind the logo,
// nested shield outlines, the logo's "A" chevron and a faint dot grid. Inline
// SVG, so it needs no image request and stays sharp at every size.
export function BrandBackdrop({ className }: { className?: string }) {
  return (
    <svg
      aria-hidden="true"
      className={cn("pointer-events-none absolute inset-0 size-full", className)}
      viewBox="0 0 800 1000"
      preserveAspectRatio="xMidYMid slice"
    >
      <defs>
        <linearGradient id="bb-field" x1="0" y1="0" x2="0.6" y2="1">
          <stop offset="0" stopColor={brand.charcoal} />
          <stop offset="1" stopColor={brand.ink} />
        </linearGradient>
        <radialGradient id="bb-glow" cx="0.5" cy="0.45" r="0.42">
          <stop offset="0" stopColor={brand.orange} stopOpacity="0.32" />
          <stop offset="0.55" stopColor={brand.ember} stopOpacity="0.08" />
          <stop offset="1" stopColor={brand.ember} stopOpacity="0" />
        </radialGradient>
        <pattern id="bb-dots" width="28" height="28" patternUnits="userSpaceOnUse">
          <circle cx="2" cy="2" r="1.2" fill={brand.cream} fillOpacity="0.07" />
        </pattern>
        <linearGradient id="bb-chevron" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor={brand.orange} stopOpacity="0.5" />
          <stop offset="1" stopColor={brand.orange} stopOpacity="0" />
        </linearGradient>
      </defs>

      <rect width="800" height="1000" fill="url(#bb-field)" />
      <rect width="800" height="1000" fill="url(#bb-dots)" />
      <rect width="800" height="1000" fill="url(#bb-glow)" />

      {/* Nested shields around the logo position (centre 400, 450). */}
      {[9, 6.6, 4.4].map((s, i) => (
        <path
          key={s}
          d={SHIELD}
          transform={`translate(${400 - 50 * s} ${450 - 55 * s}) scale(${s})`}
          fill="none"
          stroke={brand.cream}
          strokeOpacity={0.05 + i * 0.03}
          strokeWidth={1.4 / s}
          vectorEffect="non-scaling-stroke"
        />
      ))}

      {/* The logo's "A" chevron, large and fading downwards. */}
      <path
        d="M400 120 L760 1000 M400 120 L40 1000"
        stroke="url(#bb-chevron)"
        strokeWidth="3"
        fill="none"
      />
      <path d="M400 230 L690 1000 M400 230 L110 1000" stroke={brand.cream} strokeOpacity="0.05" strokeWidth="2" fill="none" />

      {/* Ember baseline. */}
      <rect y="992" width="800" height="8" fill={brand.orange} fillOpacity="0.85" />
    </svg>
  );
}

// BrandWordmark is the "AntiFox" name in brand colours: cream with an orange "Fox".
export function BrandWordmark({ className }: { className?: string }) {
  return (
    <div className={cn("font-bold tracking-tight", className)} style={{ color: brand.cream }}>
      Anti
      <span
        className="bg-clip-text text-transparent"
        style={{ backgroundImage: `linear-gradient(90deg, ${brand.orange}, ${brand.ember})` }}
      >
        Fox
      </span>
    </div>
  );
}
