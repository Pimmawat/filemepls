import { cn } from "@/lib/utils";

// Brand wordmark: Quicksand Bold (via the font-heading theme token), tight
// tracking, with "Me" picked out in the sky-blue brand accent.
export function Wordmark({ className }: { className?: string }) {
  return (
    <span
      className={cn("font-heading font-bold", className)}
      style={{ letterSpacing: "-0.7px" }}
    >
      File<span className="text-brand">Me</span>Pls
    </span>
  );
}
