import { useState } from "react";

import { attempt } from "@local/ts-shared";
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { NotesService } from "#bindings";

/** Small on purpose, so "Load more" is reachable without typing 20 notes. */
const PAGE_SIZE = 5;

const notesKey = ["notes"] as const;

/**
 * Reads and writes notes that the Go side persists in SQLite.
 *
 * The cursor contract is exactly what useInfiniteQuery wants: every page carries
 * the opaque `nextCursor` that fetches the page after it, and an empty one means
 * there is no next page. Nothing in this file knows the sort key or how the
 * cursor is built — which is the whole reason it is opaque.
 */
export function NotesPanel() {
  const [title, setTitle] = useState("");
  const queryClient = useQueryClient();

  const notes = useInfiniteQuery({
    queryKey: notesKey,
    // "" is the first page, not a missing value.
    initialPageParam: "",
    queryFn: ({ pageParam }) => NotesService.List(pageParam, PAGE_SIZE),
    // Returning undefined is how TanStack Query learns there is nothing more.
    getNextPageParam: (lastPage) => lastPage.nextCursor || undefined,
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: notesKey });

  const add = useMutation({
    // A bound Go method rejects with the Go error's message, so `attempt` turns
    // that into a value to branch on rather than an unhandled rejection.
    mutationFn: (text: string) => attempt(() => NotesService.Add(text, "")),
    onSuccess: async (result) => {
      if (!result.ok) {
        toast.error(result.error.message);
        return;
      }

      setTitle("");
      await refresh();
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => attempt(() => NotesService.Delete(id)),
    onSuccess: async (result) => {
      if (!result.ok) {
        toast.error(result.error.message);
        return;
      }

      await refresh();
    },
  });

  // `notes` is nullable in the generated type because a Go nil slice marshals
  // to null, not [].
  const rows = notes.data?.pages.flatMap((page) => page.notes ?? []) ?? [];

  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-sm font-medium text-muted-foreground">Notes (persisted in SQLite)</h2>

      <form
        className="flex gap-2"
        onSubmit={(event) => {
          event.preventDefault();
          add.mutate(title);
        }}
      >
        <input
          aria-label="Note title"
          className="flex-1 rounded-md border border-border bg-surface px-3 py-2 outline-none focus:border-brand"
          placeholder="Write a note"
          value={title}
          onChange={(event) => setTitle(event.target.value)}
        />
        <button
          className="rounded-md bg-brand px-4 py-2 font-medium text-brand-foreground disabled:opacity-50"
          disabled={add.isPending}
          type="submit"
        >
          Add
        </button>
      </form>

      {notes.isError ? (
        <p className="text-sm text-muted-foreground">Could not load notes.</p>
      ) : null}

      {!notes.isPending && rows.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No notes yet. Add one, then restart the app — it will still be here.
        </p>
      ) : null}

      <ul className="flex flex-col gap-2">
        {rows.map((note) => (
          <li
            key={note.id}
            className="flex items-center justify-between gap-3 rounded-md border border-border bg-surface px-3 py-2"
          >
            <span className="truncate">{note.title}</span>
            <button
              aria-label={`Delete ${note.title}`}
              className="text-sm text-muted-foreground hover:text-foreground disabled:opacity-50"
              disabled={remove.isPending}
              type="button"
              onClick={() => remove.mutate(note.id)}
            >
              Delete
            </button>
          </li>
        ))}
      </ul>

      {notes.hasNextPage ? (
        <button
          className="self-start rounded-md border border-border px-3 py-1.5 text-sm disabled:opacity-50"
          disabled={notes.isFetchingNextPage}
          type="button"
          onClick={() => void notes.fetchNextPage()}
        >
          {notes.isFetchingNextPage ? "Loading…" : "Load more"}
        </button>
      ) : null}
    </section>
  );
}
