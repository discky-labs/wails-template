import { attempt } from "@local/ts-shared";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { PreferencesService } from "#bindings";

/**
 * Reads and writes the preference the Go side persists to the OS config dir.
 *
 * The options come from Go too (`PreferencesService.Themes`), so the accepted
 * set is declared once in libraries/config rather than duplicated here.
 */
export function ThemeSelect() {
  const queryClient = useQueryClient();

  const preferences = useQuery({
    queryKey: ["preferences"],
    queryFn: () => attempt(() => PreferencesService.Get()),
  });

  const themes = useQuery({
    queryKey: ["themes"],
    queryFn: () => attempt(() => PreferencesService.Themes()),
    // The accepted set cannot change while the app runs.
    staleTime: Infinity,
  });

  const setTheme = useMutation({
    mutationFn: (theme: string) => attempt(() => PreferencesService.SetTheme(theme)),
    onSuccess: async (result) => {
      if (!result.ok) {
        toast.error(result.error.message);

        return;
      }

      // Go returns the stored preferences, so seed the cache from its answer
      // rather than trusting what we sent.
      queryClient.setQueryData(["preferences"], { ok: true, value: result.value });
      toast.success(`Theme saved as ${result.value.theme}`);
    },
  });

  const current = preferences.data?.ok ? preferences.data.value.theme : "";
  // A Go nil slice marshals to `null`, so the generated type is `string[] |
  // null` — coalesce rather than assume an array came back.
  const options = (themes.data?.ok ? themes.data.value : []) ?? [];

  return (
    <label className="flex items-center gap-3 text-sm">
      <span className="text-muted-foreground">Theme (persisted by Go)</span>
      <select
        className="rounded-md border border-border bg-surface px-2 py-1 outline-none focus:border-brand"
        disabled={preferences.isPending || setTheme.isPending || options.length === 0}
        value={current}
        onChange={(event) => setTheme.mutate(event.target.value)}
      >
        {options.map((theme) => (
          <option key={theme} value={theme}>
            {theme}
          </option>
        ))}
      </select>
    </label>
  );
}
