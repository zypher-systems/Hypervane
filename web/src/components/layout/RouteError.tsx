import { Button } from "@/components/ui/button";
import { Link, useRouter, type ErrorComponentProps } from "@tanstack/react-router";

/** Shown in place of a page that failed, so the rest of the app stays usable. */
export function RouteError({ error, reset }: ErrorComponentProps) {
  const router = useRouter();
  return (
    <div role="alert" className="grid h-full place-items-center p-8">
      <div className="max-w-md text-center">
        <h1 className="text-lg font-semibold">This page hit a problem</h1>
        <p className="mt-2 break-words text-sm text-ink-dim">{error.message}</p>
        <div className="mt-5 flex items-center justify-center gap-2">
          <Button
            size="sm"
            onClick={() => {
              reset();
              void router.invalidate();
            }}
          >
            Try again
          </Button>
          <Link to="/" className="rounded-md px-3 py-1.5 text-sm text-ink-dim hover:bg-raised hover:text-ink">
            Go to the overview
          </Link>
        </div>
      </div>
    </div>
  );
}
