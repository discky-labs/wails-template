import { createRootRoute, Outlet } from "@tanstack/react-router";
import { Toaster } from "sonner";

export const Route = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  return (
    <div className="flex min-h-full flex-col bg-background text-foreground">
      <Outlet />
      <Toaster theme="dark" position="bottom-center" />
    </div>
  );
}
