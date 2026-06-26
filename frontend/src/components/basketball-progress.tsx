"use client";

import { cn } from "@/lib/utils";

// A progress bar shaped like a free-throw: the ball arcs from the
// shooter on the left toward the hoop on the right as `value` advances,
// and the rim "flashes" once it reaches 100 — driven entirely by `value`
// (not a fixed-duration CSS animation), since transfer progress isn't
// linear in time.
export function BasketballProgress({ value, className }: { value: number; className?: string }) {
  const t = Math.max(0, Math.min(100, value)) / 100;
  const arcHeight = 40; // px, height of the ball's parabolic arc
  const ballX = t * 86; // % across the court, stops just short of the rim
  const ballY = arcHeight * 4 * t * (1 - t); // 0 at start/end, peak at midpoint
  const spin = t * 540; // degrees, purely decorative tumble
  const scored = t >= 1;

  return (
    <div className={cn("relative h-14 w-full overflow-hidden", className)}>
      <div className="absolute inset-x-0 bottom-1 h-px bg-border" />

      <div className="absolute right-1 top-0 flex flex-col items-center">
        <div className="h-3 w-7 rounded-t-sm border-2 border-b-0 border-orange-400/60 bg-orange-400/10" />
        <div
          className={cn(
            "h-1.5 w-6 rounded-full bg-orange-500",
            scored && "animate-[rim-flash_0.5s_ease-out]",
          )}
        />
        <div className="flex gap-0.5">
          {Array.from({ length: 4 }).map((_, i) => (
            <span
              key={i}
              className={cn(
                "h-2 w-px origin-top bg-muted-foreground/50",
                scored && "animate-[net-sway_0.5s_ease-out]",
              )}
            />
          ))}
        </div>
      </div>

      <div
        aria-hidden
        className="absolute bottom-1 left-0 text-base leading-none transition-[left,bottom] duration-150 ease-linear"
        style={{ left: `${ballX}%`, bottom: `${ballY + 4}px`, transform: `rotate(${spin}deg)` }}
      >
        🏀
      </div>
    </div>
  );
}
